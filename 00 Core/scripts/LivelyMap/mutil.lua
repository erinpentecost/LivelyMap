--[[
LivelyMap for OpenMW.
Copyright (C) 2025

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

local function getMap(data)
    if type(data) == "string" then
        -- find the full map data
        data = mapData:asTable()[data]
    elseif type(data) == "number" then
        -- find the full map data
        data = mapData:asTable()[tostring(data)]
    end
    return data
end

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

-- getScale returns a number that is the scaling factor to use
-- with this map.
-- This is used to ensure that all extents have the same in-game
-- DPI.
local function getScale(map)
    local extents = getMap(map).Extents
    -- the "default" size is 16x16 cells
    return (extents.Top - extents.Bottom) / 16
end

return {
    getMap = getMap,
    getScale = getScale,
    getClosestMap = getClosestMap,
}
