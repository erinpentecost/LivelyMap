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
local world = require('openmw.world')
local core = require('openmw.core')
local types = require('openmw.types')
local aux_util = require('openmw_aux.util')
local vfs = require('openmw.vfs')
local util = require('openmw.util')
local json = require('scripts.LivelyMap.json.json')
local mutil = require('scripts.LivelyMap.mutil')
local localization = core.l10n(MOD_NAME)
local storage = require('openmw.storage')
local mapData = storage.globalSection(MOD_NAME .. "_mapData")

-- persist is saved to disk
local persist = {
    -- map id to static record id
    -- these are created dynamically, but should be re-used
    -- this needs to be persisted in the save
    idToRecordId = {},
    -- activeMaps is a table of player -> id -> object
    -- that are currently active.
    activeMaps = {},
}

-- getMapRecord gets or creates an activator with the given mesh name.
local function getMapRecord(id)
    id = tostring(id)
    if not persist.idToRecordId[id] then
        local recordFields = {
            model = "meshes\\livelymap\\world_" .. id .. ".nif",
        }
        local draftRecord = types.Activator.createRecordDraft(recordFields)
        -- createRecord can't be used until the game is actually started.
        local record = world.createRecord(draftRecord)
        persist.idToRecordId[id] = record.id
        print("New activator record for " .. id .. ": " .. record.id)
    end
    return persist.idToRecordId[id]
end

local function newMapObject(data, player)
    local map = mutil.getMap(data)

    local record = getMapRecord(map.ID)
    if not record then
        error("No record for map " .. name)
        return nil
    end
    -- embed the owning player, too.
    map.player = player
    -- actually make the map object
    local new = world.createObject(record, 1)
    new:addScript("scripts\\LivelyMap\\mapnif.lua", map)
    map.object = new
    new:setScale(mutil.getScale(map))
    return map
end

local function onSave()
    return persist
end

local function start(data)
    -- load persist
    if data ~= nil then
        persist = data
    end
end

local function onShowMap(data)
    if not data then
        error("onShowMap has nil data parameter.")
    end
    if not data.ID then
        error("onShowMap data parameter has nil ID field.")
        return
    end
    if not data.cellID then
        error("onShowMap data parameter has nil cellID field.")
        return
    end
    if not data.position then
        error("onShowMap data parameter has nil position field.")
        return
    end
    if not data.player then
        error("onShowMap data parameter has nil player field.")
        return
    end

    local playerID = data.player.id
    if persist.activeMaps[playerID] == nil then
        persist.activeMaps[playerID] = {}
    end
    print("Showing map " .. tostring(data.ID))

    local activeMap = nil
    if persist.activeMaps[playerID][data.ID] == nil then
        -- enable the new map etc
        activeMap = newMapObject(data.ID, playerID)
        if activeMap == nil then
            error("Unknown map ID: " .. data.ID)
        end
        print("Showing new map " .. tostring(data.ID))
        persist.activeMaps[playerID][data.ID] = activeMap
    else
        -- get the existing map
        print("Moving existing map" .. tostring(data.ID))
        activeMap = persist.activeMaps[playerID][data.ID]
    end

    -- we should only show one map per player, so clean up everything else
    local toDelete = {}
    for k, v in pairs(persist.activeMaps[playerID]) do
        if k ~= data.ID then
            print("Deleting map " .. tostring(v.ID))
            activeMap.object:sendEvent(MOD_NAME .. "onMapHidden", v)
            v.object:remove()
            table.insert(toDelete, k)
        end
    end
    for _, k in ipairs(toDelete) do
        persist.activeMaps[playerID][k] = nil
    end

    -- attach the rendered object to the data
    data.object = activeMap.object

    -- teleport enables the object for free
    activeMap.object:teleport(world.getCellById(data.cellID),
        util.vector3(data.position.x, data.position.y, data.position.z), data.transform)

    -- notify the map that it moved.
    -- the map is responsible for telling the player.
    activeMap.object:sendEvent(MOD_NAME .. "onMapMoved", data)
end


return {
    eventHandlers = {
        [MOD_NAME .. "onShowMap"] = onShowMap,
    },
    engineHandlers = {
        onSave = onSave,
        onLoad = start,
        onInit = start,
    }
}
