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
local core = require('openmw.core')
local interfaces = require("openmw.interfaces")
local pself = require("openmw.self")
local aux_util = require('openmw_aux.util')

local storage = require('openmw.storage')
local stored = storage.playerSection(MOD_NAME)
stored:setLifeTime(storage.LIFE_TIME.Persistent)
local logList = {}

local persist = {
    lastCellid = nil,
    cellX = nil,
    cellY = nil,
    enterTime = nil
}

local function escape(str)
    return string.gsub(str, "([,%^%%%|'\"]+)", "_")
end

local playerName = escape(pself.type.records[pself.recordId].name)

local function log(tokenType, data)
    local line = MOD_NAME ..
        ":" .. math.ceil(core.getGameTime()) .. "," .. tokenType .. "," .. playerName .. "," .. tostring(data)
    print(line)
    table.insert(logList, math.ceil(core.getGameTime()) .. "," .. tokenType .. "," .. tostring(data))
end

local function onSave()
    print(aux_util.deepToString(logList))
    stored:set("logList", logList)
    print(aux_util.deepToString(stored:asTable()))
    return persist
end

local function onLoad(data)
    -- load working data
    if data ~= nil then
        persist = data
    end
    -- load long-term data
    logList = stored:get("logList")
    if logList == nil then
        logList = {}
    end
    print(aux_util.deepToString(logList))
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
            log("EXT",
                tostring(timeSpent) .. "," .. tostring(persist.cellX) .. "," .. tostring(persist.cellY))
        else
            log("INT", tostring(timeSpent) .. ",\"" ..
                persist.lastCellid .. "\"")
        end

        -- set up for next move
        initPersist(now)
    end
end

return {
    engineHandlers = {
        onUpdate = onUpdate,
        onSave = onSave,
        onLoad = onLoad,
    }
}
