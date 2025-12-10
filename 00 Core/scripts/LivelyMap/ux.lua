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
local mutil    = require("scripts.LivelyMap.mutil")
local core     = require("openmw.core")
local util     = require("openmw.util")
local pself    = require("openmw.self")
local aux_util = require('openmw_aux.util')
local camera   = require 'openmw.camera'


local function summonMap(id)
    local mapData
    if id == "" or id == nil then
        mapData = mutil.getClosestMap(pself.cell.gridX, pself.cell.gridY)
    else
        mapData = mutil.getMap(id)
    end

    mapData.cellID = pself.cell.id
    mapData.player = pself
    mapData.position = {
        x = pself.position.x,
        y = pself.position.y,
        z = pself.position.z,
    }
    core.sendGlobalEvent(MOD_NAME .. "onShowMap", mapData)
end

local function splitString(str)
    local out = {}
    for item in str:gmatch("([^,%s]+)") do
        table.insert(out, item)
    end
    return out
end

local function onConsoleCommand(mode, command, selectedObject)
    local function getSuffixForCmd(prefix)
        if string.sub(command:lower(), 1, string.len(prefix)) == prefix then
            return string.sub(command, string.len(prefix) + 1)
        else
            return nil
        end
    end
    local showMap = getSuffixForCmd("lua map")

    if showMap ~= nil then
        local id = splitString(showMap)
        print("Show Map: " .. aux_util.deepToString(id, 3))

        if #id == 0 then
            id = nil
        else
            id = tonumber(id[1])
        end

        summonMap(id)
    end
end

local currentMapData = nil
local function onMapMoved(data)
    print("onMapMoved" .. aux_util.deepToString(data, 3))
    -- data is a superset of the map info in maps.json
    currentMapData = data
end

-- offsetMapPos return mapPos, but shifted by the current map Extents
-- so the bottom left becomes 0,0 and top right becomes 1,1.
local function relativeMapPos(mapPos)
    if currentMapData == nil then
        error("missing mapObject")
        return
    end
    if mapPos == nil then
        error("mapPos is nil")
    end
    if currentMapData.Extents == nil then
        error("mapPos.Extents is nil")
    end
    if mapPos.x < currentMapData.Extents.Left or mapPos.x > currentMapData.Extents.Right then
        error("offsetMapPos: x position is outside extents")
    end
    if mapPos.y < currentMapData.Extents.Bottom or mapPos.y > currentMapData.Extents.Top then
        error("offsetMapPos: x position is outside extents")
    end
    local x = util.remap(mapPos.x, currentMapData.Extents.Left, currentMapData.Extents.Right, 0.0, 1.0)
    local y = util.remap(mapPos.y, currentMapData.Extents.Bottom, currentMapData.Extents.Top, 0.0, 1.0)
    return util.vector2(x, y)
end

-- relativeMapPosToWorldPos turns a relative map position to a real-world rendered position.
local function relativeMapPosToWorldPos(mapPos)
    if currentMapData == nil then
        error("no current map")
        return
    end
    if currentMapData.mapObject == nil then
        error("missing mapObject")
        return
    end
    if mapPos == nil then
        error("mapPos is nil")
    end

    -- assume the map is always not rotated
    local box = currentMapData.mapObject:getBoundingBox()
    -- just guessing these indices
    local topLeft = camera.worldToViewportVector(box.vertices[1])
    local topRight = camera.worldToViewportVector(box.vertices[2])
    local bottomLeft = camera.worldToViewportVector(box.vertices[3])
    local bottomRight = camera.worldToViewportVector(box.vertices[4])

    -- need to offset by topleft
    -- https://gitlab.com/modding-openmw/s3ctors-s3cret-st4sh/-/blob/master/h3lp_yours3lf/scripts/s3/camHelper.lua?ref_type=heads
end

-- placeIcon moves the given uxComponent so it appears on the target cell.
local function placeIcon(uxComponent, cellX, cellY)
    if currentMapData == nil then
        error("no current map")
        return
    end
end

return {
    eventHandlers = {
        [MOD_NAME .. "onMapMoved"] = onMapMoved,
    },
    engineHandlers = {
        onConsoleCommand = onConsoleCommand,
    }
}
