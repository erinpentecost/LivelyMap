--[[
LivelyMap for OpenMW.
Copyright (C) Erin Pentecost 2025

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
]]

local core                 = require("openmw.core")
local util                 = require("openmw.util")
local pself                = require("openmw.self")
local aux_util             = require('openmw_aux.util')
local postprocessing       = require('openmw.postprocessing')
local putil                = require("scripts.LivelyMap.putil")
local camera               = require("openmw.camera")

local GRID_SIZE            = 16
local GRID_ELEMS           = GRID_SIZE * GRID_SIZE
local BLEND_SPEED          = 0.3

local FogShaderFunctions   = {}
FogShaderFunctions.__index = FogShaderFunctions

---@class FogShader
---@field setCell fun(x: number, y: number, strength: number, dt: number)
---@field update fun(dt: number)
---@field setEnabled fun(status: boolean)

---@return FogShader
function NewFogShader()
    local new = {
        ---@type number[]
        fogValues = {},
        ---@type boolean
        enabled = false,
        shader = postprocessing.load("fow"),
        updateCoroutine = nil,
    }
    for i = 1, 256 do
        new.fogValues[i] = 1
    end
    setmetatable(new, FogShaderFunctions)
    return new
end

--- Convert 2D indices to 1D index (row-major, 1-based)
---@param x number  -- 1-based column
---@param y number  -- 1-based row
---@return number index
local function index2DTo1D(x, y)
    return util.clamp((y - 1) * GRID_SIZE + x, 1, GRID_ELEMS)
end

local function index1DTo2D(index)
    local y = math.floor(index / GRID_SIZE)
    local x = index % GRID_SIZE
    return x, y
end

---Set how foggy the cell is.
---@param x number
---@param y number
---@param strength number
function FogShaderFunctions.setCell(self, x, y, strength, dt)
    -- find point in 2d array
    local idx = index2DTo1D(x, y)
    -- blend in the new value
    local step = util.clamp(BLEND_SPEED * dt, 0, 1)
    self.fogValues[idx] = (strength * step) + (self.fogValues[idx] * (1 - step))
end

---@param status boolean
function FogShaderFunctions.setEnabled(self, status)
    if self.enabled == status then
        return
    end
    self.enabled = status
    if status then
        print("enabling fog shader")
        for i = 1, 256 do
            self.fogValues[i] = 1
        end
        self.shader:enable()
    else
        print("disabling fog shader")
        for i = 1, 256 do
            self.fogValues[i] = 0
        end
        self.shader:disable()
    end
end

local normalizedGridPoints = {}
local function populateNormalizedGridPoints()
    for x = 1, GRID_SIZE do
        for y = 1, GRID_SIZE do
            local idx = index2DTo1D(x, y)
            -- TODO: this might not be what the shader expects. could be off by 1
            normalizedGridPoints[idx] = util.vector2((x - 1) / GRID_SIZE, (y - 1) / GRID_SIZE)
        end
    end
end
populateNormalizedGridPoints()

---@param currentMapData MeshAnnotatedMapData
function FogShaderFunctions.updateStep(self, currentMapData, dt)
    if currentMapData == nil then
        return
    end

    local randomizedGridPoints = {}
    for i, gp in ipairs(normalizedGridPoints) do
        print(i .. " - " .. aux_util.deepToString(gp, 3))
        -- pause every ten updates
        if i % 10 == 0 then
            print("fog step " .. i .. "/" .. #normalizedGridPoints)
            coroutine.yield()
        end

        local newIdx = math.random(1, #randomizedGridPoints) + 1
        table.insert(randomizedGridPoints, newIdx, gp)

        local rel = putil.viewportPosToRelativeMeshPos(currentMapData, nil, true, gp)
        if not rel then
            print("rel is bad")
            return nil
        end

        local cellPos = putil.relativeMeshPosToCellPos(currentMapData, rel)
        if not cellPos then
            print("cellPos is nil")
            return nil
        end

        -- TODO: determine if cell pos is visible
        -- check if cellpos is in fog
        self:setCell(gp.x, gp.y, 1, dt)

        --print("update shader")
        -- update shader
        self.shader:setFloatArray("FogGrid", self.fogValues)
        print(aux_util.deepToString(self.fogValues, 3))
    end

    normalizedGridPoints = randomizedGridPoints
end

local lag = 0
---@param currentMapData MeshAnnotatedMapData
function FogShaderFunctions.update(self, currentMapData, dt)
    if currentMapData == nil then
        self:setEnabled(false)
        lag = 0
        return
    else
        self:setEnabled(true)
    end
    lag = lag + dt
    local ok
    if not self.updateCoroutine then
        print("new coroutine")
        self.updateCoroutine = coroutine.create(FogShaderFunctions.updateStep)
        ok = coroutine.resume(self.updateCoroutine, self, currentMapData, lag)
        lag = 0
    else
        ok = coroutine.resume(self.updateCoroutine)
    end
    if not ok then
        self.updateCoroutine = nil
    end
end

return {
    ---@type fun() FogShader
    NewFogShader = NewFogShader,
    GRID_SIZE = GRID_SIZE,
}
