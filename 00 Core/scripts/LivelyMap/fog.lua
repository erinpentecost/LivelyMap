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
local postprocessing       = require('openmw.postprocessing')
local putil                = require("scripts.LivelyMap.putil")

local GRID_SIZE            = 16
local GRID_ELEMS           = GRID_SIZE * GRID_SIZE
local BLEND_SPEED          = 0.3

local FogShaderFunctions   = {}
FogShaderFunctions.__index = FogShaderFunctions

---@class FogShader
---@field setCell fun(x: number, y: number, strength: number, dt: number)
---@field update fun()
---@field setEnabled fun(status: boolean)

---@return FogShader
function NewFogShader()
    local new = {
        ---@type number[]
        fogValues = {},
        ---@type boolean
        enabled = false,
        shader = postprocessing.load("fow"),
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

--- @param currentMapData MeshAnnotatedMapData
function FogShaderFunctions.update(self, currentMapData)
    if not self.enabled then
        return
    end
end

---@param status boolean
function FogShaderFunctions.setEnabled(self, status)
    if self.enabled == status then
        return
    end
    self.enabled = status
    if status then
        for i = 1, 256 do
            self.fogValues[i] = 1
        end
        self.shader:enable()
    else
        for i = 1, 256 do
            self.fogValues[i] = 0
        end
        self.shader:disable()
    end
end

--- @param currentMapData MeshAnnotatedMapData
function FogShaderFunctions.update(self, currentMapData)
    if currentMapData == nil then
        self:setEnabled(false)
        return
    else
        self:setEnabled(true)
    end

    --- update fog values
    for x = 1, GRID_SIZE do
        for y = 1, GRID_SIZE do
            local rel = putil.viewportPosToRelativeMeshPos(currentMapData, viewportPos, true)
            if not rel then
                return nil
            end

            -- 4. Relative mesh → cell
            local cellPos = putil.relativeMeshPosToCellPos(currentMapData, rel)
            if not cellPos then
                print("cellPos is nil")
                return nil
            end
        end
    end

    self.shader:setFloatArray("fogGrid", self.fogValues)
end

return {
    ---@type fun() FogShader
    NewFogShader = NewFogShader,
    GRID_SIZE = GRID_SIZE,
}
