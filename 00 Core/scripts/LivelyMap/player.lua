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

local persist = {
    id = playerName(),
    paths = {},
}

local function onSave()
    -- debug
    print("onSave:" .. aux_util.deepToString(persist, 3))
    -- do the save. this needs to be in json
    -- so the Go code can unmarshal it.
    return { json = wrapInMagic(json.encode(persist)) }
end

local function onLoad(data)
    -- load from in-game storage
    if data ~= nil then
        persist = json.decode(unwrapMagic(data.json))
    end
    -- TODO: merge with dumped JSON file from Go code.

    -- debug
    print("onLoad:" .. aux_util.deepToString(persist, 3))
end

local function addEntry()
    local entry = {
        t = math.ceil(core.getGameTime()),
        x = pself.cell.isExterior and pself.cell.gridX or nil,
        y = pself.cell.isExterior and pself.cell.gridY or nil,
        c = (not pself.cell.isExterior) and pself.cell.id or nil
    }

    -- make a new list and add the entry to it
    if persist == nil or #persist.paths == 0 then
        persist = {
            id = playerName(),
            paths = { entry }
        }
        print("Initialized new local storage with entry: " .. aux_util.deepToString(entry, 3))
        return
    end
    -- otherwise, don't do anything if the cell is the same.
    local tail = persist.paths[#persist.paths]
    if tail.c == entry.c and tail.x == entry.x and tail.y == entry.y then
        return
    end
    -- ok, now add to the end of the list.
    table.insert(persist.paths, entry)
    print("Added new entry: " .. aux_util.deepToString(entry, 3))
end

local function onUpdate(dt)
    if dt == 0 then
        -- don't do anything if paused.
        return
    end
    addEntry()
end

return {
    engineHandlers = {
        onUpdate = onUpdate,
        onSave = onSave,
        onLoad = onLoad,
    }
}
