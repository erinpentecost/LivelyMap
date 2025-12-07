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
local pself    = require("openmw.self")
local aux_util = require('openmw_aux.util')

local function summonMap(id)
    if id == "" or id == nil then
        local closest = mutil.getClosestMap(pself.cell.gridX, pself.cell.gridY)
        id = closest.ID
    else
        id = tonumber(id)
    end

    local data = {
        ID = id,
        cellID = pself.cell.id,
        player = pself,
        position = {
            x = pself.position.x,
            y = pself.position.y,
            z = pself.position.z,
        },
    }
    core.sendGlobalEvent(MOD_NAME .. "onShowMap", data)
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

local function onMapMoved(data)
    print("onMapMoved" .. aux_util.deepToString(data, 3))
end

return {
    eventHandlers = {
        [MOD_NAME .. "onMapMoved"] = onMapMoved,
    },
    engineHandlers = {
        onConsoleCommand = onConsoleCommand,
    }
}
