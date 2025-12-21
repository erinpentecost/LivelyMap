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
local MOD_NAME = require("scripts.LivelyMap.ns")
local storage = require('openmw.storage')
local mapData = storage.globalSection(MOD_NAME .. "_mapData")
local util = require('openmw.util')

-- https://github.com/LuaLS/lua-language-server/wiki/Annotations

---@class HasID
---@field ID number

---@class Connection
---@field east number?
---@field west number?
---@field south number?
---@field north number?

---@class Extents
---@field Top number
---@field Bottom number
---@field Left number
---@field Right number

---@class StoredMapData : HasID
---@field ID number
---@field Extents Extents
---@field ConnectedTo Connection
---@field CenterX number
---@field CenterY number

---Returns immutable map metadata.
---@param data string | number | HasID
---@return StoredMapData
local function getMap(data)
    if type(data) == "string" then
        -- find the full map data
        return mapData:asTable()[data]
    elseif type(data) == "number" then
        -- find the full map data
        return mapData:asTable()[tostring(data)]
    elseif type(data) == "table" then
        return mapData:asTable()[tostring(data.ID)]
    end
    error("getMap: unknown type")
end

---Returns immutable map metadata for the map closest to the provided cell coordinates.
---@param x number Cell grid X.
---@param y number Cell grid y.
---@return StoredMapData
local function getClosestMap(x, y)
    local myLocation = util.vector2(x, y)
    local closest = nil
    local closestDist = 0
    for _, v in pairs(mapData:asTable()) do
        local thisDist = (util.vector2(v.CenterX, v.CenterY) - myLocation):length2()
        if closest == nil then
            closest = v
            closestDist = thisDist
        else
            if thisDist < closestDist then
                closest = v
                closestDist = thisDist
            end
        end
    end
    return closest
end

--- getScale returns a number that is the scaling factor to use
--- with this map.
--- This is used to ensure that all extents have the same in-game
--- DPI.
---@param map StoredMapData
---@return number
local function getScale(map)
    local extents = getMap(map).Extents
    -- the "default" size is 16x16 cells
    return (extents.Top - extents.Bottom) / 16
end

local function lerpVec3(a, b, t)
    return util.vector3(
        a.x + (b.x - a.x) * t,
        a.y + (b.y - a.y) * t,
        a.z + (b.z - a.z) * t
    )
end

local function lerpVec2(a, b, t)
    return util.vector2(
        a.x + (b.x - a.x) * t,
        a.y + (b.y - a.y) * t
    )
end

local CELL_SIZE = 64 * 128 -- 8192

---@param worldPos WorldSpacePos
---@return CellPos
local function worldPosToCellPos(worldPos)
    if worldPos == nil then
        error("worldPos is nil")
    end

    --- Position in world space, but units have been changed to match cell lengths.
    --- To get the cell grid position, take the floor of these elements.
    return util.vector3(worldPos.x / CELL_SIZE, worldPos.y / CELL_SIZE, worldPos.z / CELL_SIZE)
end

---@param cellPos CellPos
---@return WorldSpacePos
local function cellPosToWorldPos(cellPos)
    if cellPos == nil then
        error("cellPos is nil")
    end

    return util.vector3(cellPos.x * CELL_SIZE, cellPos.y * CELL_SIZE, cellPos.z * CELL_SIZE)
end

local function inBox(position, box)
    local normalized = box.transform:inverse():apply(position)
    return math.abs(normalized.x) <= 1
        and math.abs(normalized.y) <= 1
        and math.abs(normalized.z) <= 1
end

---@param data table
---@param ... table
---@return table
local function shallowMerge(data, ...)
    local copy = {}
    for k, v in pairs(data) do
        copy[k] = v
    end
    local arg = { ... }
    for _, extraData in ipairs(arg) do
        for k, v in pairs(extraData) do
            copy[k] = v
        end
    end
    return copy
end

return {
    CELL_SIZE = CELL_SIZE,
    getMap = getMap,
    getScale = getScale,
    getClosestMap = getClosestMap,
    lerpVec3 = lerpVec3,
    lerpVec2 = lerpVec2,
    worldPosToCellPos = worldPosToCellPos,
    cellPosToWorldPos = cellPosToWorldPos,
    inBox = inBox,
    shallowMerge = shallowMerge,
}
