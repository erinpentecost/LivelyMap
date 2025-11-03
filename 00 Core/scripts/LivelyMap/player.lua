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

local lastCellid
local cellX
local cellY
local enterTime

local function escape(str)
    return string.gsub(str, "([,%^%%%|'\"]+)", "_")
end

local playerName = escape(pself.type.records[pself.recordId].name)

local function log(tokenType, data)
    print(MOD_NAME ..
        ":" .. math.ceil(core.getGameTime()) .. "," .. tokenType .. "," .. playerName .. "," .. tostring(data))
end

local function onSave()
    return {
        lastCellid = lastCellid,
        enterTime = enterTime,
        cellX = cellX,
        cellY = cellY,
    }
end

local function onLoad(data)
    log("START_SESSION")
    if data ~= nil then
        lastCellid = data.lastCellid
        enterTime = data.enterTime
        cellX = data.cellX
        cellY = data.cellY
    end
end

local function onUpdate(dt)
    local now = core.getGameTime()

    if lastCellid == nil then
        lastCellid = pself.cell.id
        enterTime = now
    end
    if lastCellid ~= pself.cell.id then
        local timeSpent = math.ceil(now - enterTime)
        if cellX ~= nil then
            -- this is an exterior
            log("TRACK_EXTERIOR",
                tostring(timeSpent) .. "," .. tostring(cellX) .. "," .. tostring(cellY) .. ",\"" ..
                lastCellid .. "\"")
        else
            log("TRACK_INTERIOR", tostring(timeSpent) .. ",\"" ..
                lastCellid .. "\"")
        end

        -- set up for next move
        enterTime = now
        lastCellid = pself.cell.id
        if pself.cell.isExterior then
            cellX = pself.cell.gridX
            cellY = pself.cell.gridY
        else
            cellX = nil
            cellY = nil
        end
    end
end

return {
    engineHandlers = {
        onUpdate = onUpdate,
        onSave = onSave,
        onLoad = onLoad,
    }
}
