local MOD_NAME = require("scripts.LivelyMap.ns")
local world = require('openmw.world')
local core = require('openmw.core')
local types = require('openmw.types')
local aux_util = require('openmw_aux.util')
local vfs = require('openmw.vfs')
local util = require('openmw.util')
local json = require('scripts.LivelyMap.json.json')
local localization = core.l10n(MOD_NAME)



-- persist is saved to disk
local persist = {
    -- map name to static record id
    -- these are created dynamically, but should be re-used
    -- this needs to be persisted in the save
    meshToRecordId = {},
    -- map name to object instance id.
    -- just enable/disable these to show them
    meshToObject = {},
}

-- getMapRecord gets or creates an activator with the given mesh name.
local function getMapRecord(name)
    if not persist.meshToRecordId[name] then
        local recordFields = {
            name = localization("map", {}),
            model = "meshes\\livelymap\\" .. name .. ".nif",
        }
        local draftRecord = types.Activator.createRecordDraft(recordFields)
        local record = world.createRecord(draftRecord)
        persist.meshToRecordId[name] = record.id
        print("New activator for " .. name .. ": " .. record.id)
    end
    return persist.meshToRecordId[name]
end

local function getMapObject(name)
    if not persist.meshToObject[name] then
        local record = getMapRecord(name)
        if not record then
            error("No record for map " .. name)
            return
        end
        return world.createObject(record, 1)
    end
    return persist.meshToObject[name]
end

-- this is derived from maps.json.
-- it's a map of map infos (not an array)
-- and there's an additional "object" field.
local maps = {}

local function onSave()
    return persist
end

local function onLoad(data)
    -- load persist
    if data ~= nil then
        persist = data
    end

    -- load from file
    local path = "scripts\\" .. MOD_NAME .. "\\data\\maps.json"
    print("onLoad: Started. Path file: " .. path)
    local handle, err = vfs.open(path)
    if handle == nil then
        error("OnLoad: Failed to read " .. path .. " - " .. tostring(err))
        return
    end

    -- augment maps with object
    -- also turn it into a map instead of array
    maps = {}
    local mapsList = json.decode(handle:read("*all")).Maps
    for _, v in ipairs(mapsList) do
        print("Parsing map: " .. v.ID)
        v.object = getMapObject("world_" .. v.ID)
        maps[v.ID] = v
    end
    print("Maps: " .. aux_util.deepToString(maps, 3))
end


local activeMap = nil
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
    if activeMap and activeMap.ID == data.ID then
        -- this is the same map. don't make a new one.
        return
    elseif activeMap then
        -- delete the current map
        activeMap.object.enabled = false
    end

    -- enable the new map etc
    activeMap = maps[data.ID]
    if activeMap == nil then
        error("Unknown map ID: " .. data.ID)
    end

    -- teleport enables the object for free
    activeMap.object:teleport(world.getCellById(data.cellID), util.vector3(data.position.x, data.position.y, data.position.z), data.transform)
end


return {
    eventHandlers = {
        [MOD_NAME .. "onShowMap"] = onShowMap,
    },
    engineHandlers = {
        onSave = onSave,
        onLoad = onLoad,
        onInit = function() onLoad(nil) end,
    }
}
