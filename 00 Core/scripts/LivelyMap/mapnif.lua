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
local aux_util = require('openmw_aux.util')

-- This script is attached to the 3d floating map objects.
-- It should be responsible for adding annotations and cleaning
-- them up.

-- mapData holds read-only map metadata for this extent.
-- It also contains "player", which is the player that
-- summoned it.
local mapData = nil

local function onStart(initData)
    if initData ~= nil then
        mapData = initData
    end
end
local function onSave()
    return mapData
end

local function onActive()
    print("activated " .. aux_util.deepToString(mapData, 3))
end

local function onInactive()
    -- this is called after the map is destroyed,
    -- so it's probably too late to clean up annotations at this point.
    print("inactivated " .. aux_util.deepToString(mapData, 3))
end

-- onTeleported is called when the map is placed or moved.
-- this should move/create all the annotations it owns
local function onTeleported(data)
    if data == nil then
        error("onTeleported data is nil")
    end
    if data.position == nil then
        error("onTeleported data.position is nil")
    end
    mapData = data
    print("onTeleported")

    mapData.player:sendEvent(MOD_NAME .. "onMapMoved", data)
end

return {
    eventHandlers = {
        [MOD_NAME .. "onTeleported"] = onTeleported,
    },
    engineHandlers = {
        onActive = onActive,
        onInactive = onInactive,
        onInit = onStart,
        onSave = onSave,
        onLoad = onStart,
    }
}
