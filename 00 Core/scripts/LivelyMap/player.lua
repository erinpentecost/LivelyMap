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
local util = require('openmw.util')
local types = require('openmw.types')
local json = require('scripts.LivelyMap.json.json')
local core = require('openmw.core')
local interfaces = require("openmw.interfaces")
local pself = require("openmw.self")
local vfs = require('openmw.vfs')
local aux_util = require('openmw_aux.util')

local storage = require('openmw.storage')

local magicPrefix = "!!" .. MOD_NAME .. "!!STARTOFENTRY!!"
local magicSuffix = "!!" .. MOD_NAME .. "!!ENDOFENTRY!!"

local function wrapInMagic(str)
    return magicPrefix .. str .. magicSuffix
end
local function unwrapMagic(str)
    return string.sub(str, #magicPrefix + 1, #str - #magicSuffix)
end

local function playerName()
    return types.NPC.record(pself.recordId).name
end

-- Merge two SaveData-like tables: a and b
-- Returns a new table shaped like SaveData
local function merge(a, b)
    if a == nil then return b end
    if b == nil then return a end
    local apaths = a.paths or {}
    local bpaths = b.paths or {}

    -- If a or b is empty, easy cases
    if #apaths == 0 then
        return { id = b.id, paths = bpaths }
    end
    if #bpaths == 0 then
        return { id = a.id, paths = apaths }
    end

    -- Find newest timestamp in b
    local b_newest = bpaths[#bpaths].t

    -- Copy from a until timestamps overlap
    local result_paths = {}
    for i = 1, #apaths do
        if apaths[i].t >= b_newest then
            break
        end
        result_paths[#result_paths + 1] = apaths[i]
    end

    -- Append all of b
    for i = 1, #bpaths do
        result_paths[#result_paths + 1] = bpaths[i]
    end

    return {
        id = b.id or a.id,
        paths = result_paths
    }
end

-- fromSave contains the data from this savegame.
local fromSave = {
    id = playerName(),
    paths = {},
    extra = {},
}

-- mergedData contains the merged data from the savegame and file,
-- for all saves. The key is the playerName.
local allData = {}
allData[playerName()] = {
  id = playerName(),
  paths = {},
  extra = {},
}

local function onSave()
    -- debug
    print("onSave:" .. aux_util.deepToString(fromSave, 3))
    -- do the save. this needs to be in json
    -- so the Go code can unmarshal it.
    return { json = wrapInMagic(json.encode(fromSave)) }
end

local function newEntry()
    return {
        t = math.ceil(core.getGameTime()),
        x = pself.cell.isExterior and pself.cell.gridX or nil,
        y = pself.cell.isExterior and pself.cell.gridY or nil,
        c = (not pself.cell.isExterior) and pself.cell.id or nil
    }
end

local function parseFile(path)
    local handle, err = vfs.open(path)
    if handle == nil then
        print("OnLoad: Failed to read " .. path .. " - " .. tostring(err))
        allData[playerName()] = fromSave
        return
    end
    return json.decode(handle:read("*all"))
end

local function endsWith(str, ending)
  if ending == "" then
    return true
  end
  if #str < #ending then
    return false
  end
  return str:sub(-#ending) == ending
end

local function onLoad(data)
    local path = "scripts\\" .. MOD_NAME .. "\\data\\paths\\" .. playerName() .. ".json"
    print("onLoad: Started. Path file: " .. path)

    -- load from in-game storage
    if data ~= nil then
        fromSave = json.decode(unwrapMagic(data.json))
    end

    -- load from file. this is produced by the Go portion of the mod.
    local fromFile = parseFile(path)

    -- merge them
    allData[playerName()] = merge(fromFile, fromSave)

    -- debug
    print("onLoad: " .. aux_util.deepToString(allData, 3))

    -- now load all other character data
    local allSaves = "scripts\\" .. MOD_NAME .. "\\data\\paths\\"
    for fileName in vfs.pathsWithPrefix(allSaves) do
        if fileName ~= path and endsWith(fileName, ".json") then
            -- this is for a different character
            local lastSlash = math.max(path:find("/", 1, true) or 0, path:find("\\", 1, true) or 0)
            local characterName = fileName:sub(lastSlash):gsub("%.json", "")
            if not allData[characterName] then
              allData[characterName] = parseFile(path)
              print("onLoad completed for " .. characterName)
            end
        end
    end
end

local function addEntry()
    local entry = newEntry()
    name = playerName()

    -- make a new list and add the entry to it
    if allData[name] == nil or #allData[name].paths == 0 then
        mergedData = {
            id = playerName(),
            paths = { entry }
        }
        print("Initialized new local storage with entry: " .. aux_util.deepToString(entry, 3))
        return
    end
    -- otherwise, don't do anything if the cell is the same.
    local tail = allData[name].paths[#allData[name].paths]
    if tail.c == entry.c and tail.x == entry.x and tail.y == entry.y then
        return
    end
    -- ok, now add to the end of the list.
    table.insert(allData[name].paths, entry)
    table.insert(fromSave.paths, entry)
    print("Added new entry: " .. aux_util.deepToString(entry, 3))
end

local cachedCellId
local function onUpdate(dt)
    if dt == 0 then
        -- don't do anything if paused.
        return
    end
    if pself.cell.id == cachedCellId then
        return
    else
        cachedCellId = pself.cell.id
    end
    addEntry()
end

return {
    interfaceName = MOD_NAME.."Path",
    interface = {
        version = 1,
        getPaths = function() return allData end,
    },
    engineHandlers = {
        onUpdate = onUpdate,
        onSave = onSave,
        onLoad = onLoad,
        onInit = function() onLoad(nil) end,
    }
}
