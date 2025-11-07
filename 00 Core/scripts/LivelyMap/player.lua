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

-- TODO: either don't use storage sections,
-- or use global storage. never player storage.
--
-- Global storage would let us read the data regardless of current player,
-- so we could show all local history.
--
-- Saving into the player object with onSave/onLoad would bind the data
-- into the player omwsave file, which would make it transferable.

local MOD_NAME = require("scripts.LivelyMap.ns")
local util = require('openmw.util')
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

local logList = {}

local persist = {
    lastCellid = nil,
    cellX = nil,
    cellY = nil,
    enterTime = nil,
    jsonLog = nil
}

local function log(data)
    print(aux_util.deepToString(data))
    table.insert(logList, data)
end

local function onSave()
    -- debug
    print("onSave:" .. aux_util.deepToString(logList, 3))
    -- do the save
    persist.jsonLog = wrapInMagic(json.encode(logList))
    return persist
end

local function onLoad(data)
    -- load working data
    if data ~= nil then
        persist = data
    end
    -- load long-term data
    logList = json.decode(unwrapMagic(data.jsonLog))
    -- debug
    print("onLoad:" .. aux_util.deepToString(logList, 3))
end

local function initPersist(now)
    persist.enterTime = now
    persist.lastCellid = pself.cell.id
    if pself.cell.isExterior then
        persist.cellX = pself.cell.gridX
        persist.cellY = pself.cell.gridY
    else
        persist.cellX = nil
        persist.cellY = nil
    end
end

local function onUpdate(dt)
    if dt == 0 then
        return
    end
    local now = core.getGameTime()

    if persist.lastCellid == nil then
        initPersist(now)
    end
    if persist.lastCellid ~= pself.cell.id then
        local timeSpent = math.ceil(now - persist.enterTime)
        if persist.cellX ~= nil then
            -- this is an exterior
            log({
                -- timestamp
                t = math.ceil(core.getGameTime()),
                -- duration
                d = timeSpent,
                -- x pos
                x = persist.cellX,
                -- y pos
                y = persist.cellY,
            })
        else
            log({
                -- timestamp
                t = math.ceil(core.getGameTime()),
                -- duration
                d = timeSpent,
                -- interior id
                id = persist.lastCellid,
            })
        end

        -- set up for next move
        initPersist(now)
    end
end

local function onConsoleCommand(mode, command, selectedObject)
    local function getSuffixForCmd(prefix)
        if string.sub(command:lower(), 1, string.len(prefix)) == prefix then
            return string.sub(command, string.len(prefix) + 1)
        else
            return nil
        end
    end
    local noTrespass = getSuffixForCmd("lua clearmap")

    if noTrespass ~= nil then
        print("Clearing data in " .. MOD_NAME .. ". You must save to confirm.")
        logList = {}
    end
end

return {
    engineHandlers = {
        onUpdate = onUpdate,
        onSave = onSave,
        onLoad = onLoad,
        onConsoleCommand = onConsoleCommand,
    }
}
