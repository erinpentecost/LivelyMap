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

---@class GloballyAnnotatedMapData : StoredMapData
---@field player userdata The player that owns this instance.
---@field object userdata The map mesh static object instance.
---@field swapped boolean? Indicates the swap-in or swap-out state of the map.

---Returns immutable map metadata.
---@param data string | number | HasID
---@param player userdata
---@return GloballyAnnotatedMapData?
local function newMapObject(data, player)
    local map = mutil.getMap(data)

    local record = getMapRecord(map.ID)
    if not record then
        error("No record for map " .. map.ID)
        return nil
    end

    -- actually make the map object
    local new = world.createObject(record, 1)
    new:addScript("scripts\\LivelyMap\\mapnif.lua", map)

    local extra = {
        player = player,
        object = new,
    }

    -- scale the object
    new:setScale(mutil.getScale(map))

    return mutil.shallowMerge(map, extra)
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
    if not data.cellID then
        error("onShowMap data parameter has nil cellID field.")
        return
    end
    if not data.player then
        error("onShowMap data parameter has nil player field.")
        return
    end
    if type(data.player) == "string" then
        error("onShowMap data parameter has a string player field.")
        return
    end

    if (not data.ID) and (not data.position) then
        -- One of these two are required.
        -- position is world position.
        -- ID is the map ID in maps.json.
        error("onShowMap data parameter has nil ID and nil position field.")
        return
    end

    -- Find ID or startWorldPosition based on the other one.
    if not data.startWorldPosition and not data.ID then
        if data.startWorldPosition then
            local cellPos = mutil.worldPosToCellPos(data.startWorldPosition)
            data.ID = mutil.getClosestMap(math.floor(cellPos.x), math.floor(cellPos.y))
        else
            local mapdata = mutil.getMap(data)
            data.startWorldPosition = util.vector3(mapdata.CenterX * mutil.CELL_SIZE, mapdata.CenterY * mutil.CELL_SIZE,
                0)
        end
    end

    local mapPosition = data.mapPosition or
        util.vector3(data.player.position.x, data.player.position.y, data.player.position.z + 5 * mutil.CELL_SIZE)

    local playerID = data.player.id
    if persist.activeMaps[playerID] == nil then
        persist.activeMaps[playerID] = {}
    end
    print("Showing map " .. tostring(data.ID))

    local activeMap = nil
    if persist.activeMaps[playerID][data.ID] == nil then
        -- enable the new map etc
        activeMap = newMapObject(data.ID, data.player)
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
    local swapped = false
    local toDelete = {}
    for k, v in pairs(persist.activeMaps[playerID]) do
        if k ~= data.ID then
            swapped = true
            print("Deleting map " .. tostring(v.ID))
            -- swapped means the map is being replaced with a different one.
            v.player:sendEvent(MOD_NAME .. "onMapHidden",
                mutil.shallowMerge(v, {
                    swapped = swapped
                }))
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
        mapPosition,
        nil)

    -- notify the map that it moved.
    -- the map is responsible for telling the player.
    activeMap.object:sendEvent(MOD_NAME .. "onMapMoved",
        mutil.shallowMerge(data, { swapped = swapped }))
end

local function onHideMap(data)
    if not data then
        error("onShowMap has nil data parameter.")
    end
    if not data.player then
        error("onShowMap data parameter has nil player field.")
        return
    end

    local playerID = data.player.id
    if persist.activeMaps[playerID] == nil then
        persist.activeMaps[playerID] = {}
    end

    local toDelete = {}
    for k, v in pairs(persist.activeMaps[playerID]) do
        print("Deleting map " .. tostring(v.ID) .. ": " .. aux_util.deepToString(v, 3))
        v.player:sendEvent(MOD_NAME .. "onMapHidden",
            mutil.shallowMerge(v, { swapped = false }))
        v.object:remove()
        table.insert(toDelete, k)
    end
    for _, k in ipairs(toDelete) do
        persist.activeMaps[playerID][k] = nil
    end
end

---comment
---@param cell any cell
---@return any? object in the cell
local function getRepresentiveObjectForCell(cell)

end

---@class AugmentedPos
---@field pos util.vector3
---@field object any? either the object that we requested the position of, or a representative object in an exterior space (like a door).

--- cache of interior cell id to exterior position
---@type {[string]: AugmentedPos}
local cachedPos = {}

--- Find the player's exterior location.
--- If they are in an interior, find a door to an exit and use that position.
---@param data any player or cell
---@return AugmentedPos?
local function getExteriorLocation(data)
    local inputCell = data.cell or data
    if inputCell.isExterior then
        if data.position then
            --- don't cache the easy case.
            return { pos = data.position, object = data }
        elseif cachedPos[inputCell.id] then
            -- return previously-cached computed position
            return cachedPos[inputCell.id]
        else
            --- we were passed in an exterior cell, which doesn't have a high-def
            --- world position
            local rep = getRepresentiveObjectForCell(inputCell)
            if rep then
                cachedPos[inputCell.id] = { pos = rep.position, object = rep }
            else
                --- we found no good object, so just use the center of the cell.
                cachedPos[inputCell.id] = {
                    pos = util.vector3(
                        (inputCell.gridX + 0.5) * mutil.CELL_SIZE,
                        (inputCell.gridY + 0.5) * mutil.CELL_SIZE,
                        0
                    ),
                }
            end
            return cachedPos[inputCell.id]
        end
    end
    if cachedPos[inputCell.id] then
        return cachedPos[inputCell.id]
    end
    -- we need to recurse out until we find the exit door
    local seenCells = {}
    ---@type fun(cell : any): AugmentedPos?
    local searchForDoor
    searchForDoor = function(cell)
        if not cell then
            return nil
        end
        if seenCells[cell.id] then
            return nil
        end
        seenCells[cell.id] = true
        for _, door in ipairs(cell:getAll(types.Door)) do
            local destCell = types.Door.destCell(door)
            if destCell then
                -- If this door leads directly outside, we're done
                if destCell.isExterior then
                    return { pos = types.Door.destPosition(door), object = door }
                end

                -- Otherwise, recurse
                local result = searchForDoor(destCell)
                if result then
                    return result
                end
            end
        end
        return nil
    end
    cachedPos[inputCell.id] = searchForDoor(inputCell)
    return cachedPos[inputCell.id]
end

--- Special marker handling
local function broadcastMarker(data)
    for _, player in ipairs(world.players) do
        player:sendEvent(MOD_NAME .. "onMarkerActivated", data)
    end
end
local cachedMarkers = {}
local markerRecords = {
    northmarker = true,
    templemarker = true,
    divinemarker = true,
    prisonmarker = true,
    travelmarker = true,
}
local function onObjectActive(object)
    if (not object.type) or (markerRecords[object.recordId]) then
        -- openmw hack to get a NorthMarker reference.
        -- NorthMarkers aren't available with cell:getAll().
        -- Thanks S3ctor for the workaround. :)
        if not cachedMarkers[object.cell.id] then
            cachedMarkers[object.cell.id] = {}
        end
        table.insert(cachedMarkers[object.cell.id], object)
        broadcastMarker(object)
    end
end
local function getMarkers(cell)
    if cachedMarkers[cell.id] then
        return cachedMarkers[cell.id]
    else
        return {}
    end
end

local exteriorNorth = util.transform.identity
local function getFacing(player)
    if not player.rotation then
        print("no rotation for " .. tostring(player) .. ", assuming default " .. tostring(exteriorNorth))
        return exteriorNorth
    end
    -- Player forward vector
    local forward = player.rotation:apply(util.vector3(0.0, 1.0, 0.0)):normalize()
    local northMarker = exteriorNorth
    for _, o in ipairs(getMarkers(player.cell)) do
        --print(o)
        --print(o.recordId)
        if o.recordId == "northmarker" then
            northMarker = o.rotation:inverse()
        end
    end

    -- Rotate into cardinal space
    local cardinal = northMarker * forward
    --print("northMarker: " .. aux_util.deepToString(northMarker, 3) .. ", forward: " .. tostring(forward))

    -- Project to 2D
    local v = util.vector2(cardinal.x, cardinal.y)
    return v:length() > 0 and v:normalize() or v
end


--- This is a helper to get cell information for the player,
--- since cell:getAll isn't available on local scripts.
local function onGetExteriorLocation(data)
    local posObj = getExteriorLocation(data.object)
    local facing = getFacing(data.object)
    data.callbackObject:sendEvent(MOD_NAME .. "onReceiveExteriorLocation",
        {
            pos = posObj and posObj.pos and { x = posObj.pos.x, y = posObj.pos.y, z = posObj.pos.z },
            object = posObj and posObj.object,
            facing = { x = facing.x, y = facing.y, z = facing.z },
            args = data,
        })
end

return {
    eventHandlers = {
        [MOD_NAME .. "onShowMap"] = onShowMap,
        [MOD_NAME .. "onHideMap"] = onHideMap,
        [MOD_NAME .. "onGetExteriorLocation"] = onGetExteriorLocation,
    },
    engineHandlers = {
        onSave = onSave,
        onLoad = start,
        onInit = start,
        onObjectActive = onObjectActive,
    }
}
