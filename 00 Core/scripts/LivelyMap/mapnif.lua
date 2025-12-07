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
local aux_util = require('openmw_aux.util')

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
    print("inactivated " .. aux_util.deepToString(mapData, 3))
end


return {
    engineHandlers = {
        onActive = onActive,
        onInactive = onInactive,
        onInit = onStart,
        onSave = onSave,
        onLoad = onStart,
    }
}
