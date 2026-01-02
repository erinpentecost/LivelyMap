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
local MOD_NAME     = require("scripts.LivelyMap.ns")
local mutil        = require("scripts.LivelyMap.mutil")
local putil        = require("scripts.LivelyMap.putil")
local core         = require("openmw.core")
local util         = require("openmw.util")
local pself        = require("openmw.self")
local types        = require("openmw.types")
local aux_util     = require('openmw_aux.util')
local camera       = require("openmw.camera")
local ui           = require("openmw.ui")
local settings     = require("scripts.LivelyMap.settings")
local async        = require("openmw.async")
local interfaces   = require('openmw.interfaces')
local storage      = require('openmw.storage')
local input        = require('openmw.input')
local heightData   = storage.globalSection(MOD_NAME .. "_heightData")
local keytrack     = require("scripts.LivelyMap.keytrack")
local uiInterface  = require("openmw.interfaces").UI
local localization = core.l10n(MOD_NAME)

local cellData     = storage.globalSection(MOD_NAME .. "_cellNames"):asTable()

local settingCache = {
    autoMarkNamedExteriorCells = settings.automatic.autoMarkNamedExteriorCells,
    autoMarkFromJournal = settings.automatic.autoMarkFromJournal
}

settings.automatic.subscribe(async:callback(function(_, key)
    settingCache[key] = settings.automatic[key]
end))


---@param cellName string
---@param cellId string?
local function markArea(cellName, cellId)
    if not cellId then
        cellId = cellData[cellName]
    end
    if not cellId then
        error("markArea given bad cell name: " .. tostring(cellName))
    end

    local markId = cellId .. "_area"
    local exists = interfaces.LivelyMapMarker.getMarkerByID(markId)
    if exists then
        -- don't re-mark areas.
        return
    end

    core.sendGlobalEvent(MOD_NAME .. "onGetExteriorLocation", {
        cellName = cellName,
        cellId = cellId,
        callbackObject = pself,
        source = MOD_NAME .. "_areamarker.lua",
    })
end

local function makeTemplate(cellId, cellName)
    --- TODO: try to find a good stamp for the type of cell.
    return {
        iconName = "circle",
        color = 4,
    }
end

local function onReceiveExteriorLocation(data)
    if not data then
        return
    end
    if data.args.source ~= MOD_NAME .. "_areamarker.lua" then
        return
    end

    local cellId = data.args.cellId
    local cellName = data.args.cellName

    if not cellName then
        error("onReceiveExteriorLocation bad data: " .. aux_util.deepToString(data, 3))
    end

    local markId = cellId .. "_area"

    local template = makeTemplate(cellId, cellName)

    ---@type MarkerData
    local markerInfo = {
        --- cell name
        id = markId,
        note = cellName,
        iconName = template.iconName,
        color = template.color,
        worldPos = util.vector3(data.pos.x, data.pos.y, data.pos.z),
        hidden = false,
    }
    interfaces.LivelyMapMarker.upsertMarkerIcon(markerInfo)
end


local function markFromJournal()
    -- build map of canonicalized topics
    local topics = {}
    for _, topic in pairs(types.Player.journal(pself).topics) do
        if #topic.entries > 0 then
            topics[mutil.canonicalizeId(topic.name)] = true
        end
    end

    for cellName, cellId in pairs(cellData) do
        if topics[mutil.canonicalizeId(cellName)] then
            print("Found journal entry for '" .. cellName .. "', marking it on the map.")
            markArea(cellName, cellId)
        end
    end
end

interfaces.LivelyMapDraw.onMapMoved(function(data)
    if (not data.swapped) and (settingCache.autoMarkFromJournal) then
        markFromJournal()
    end
end)

local lastExteriorCell = nil
local delta = 0
local function onUpdate(dt)
    if dt == 0 then
        -- don't do anything if paused.
        return
    end
    if not settingCache.autoMarkNamedExteriorCells then
        return
    end
    delta = delta - dt
    if delta > 0 then
        return
    end
    delta = 1.3
    if pself.cell.isExterior and lastExteriorCell ~= pself.cell then
        lastExteriorCell = pself.cell
        markArea(pself.cell.name, pself.cell.id)
    end
end

return {
    interfaceName = MOD_NAME .. "AreaMarker",
    interface = {
        version = 1,
        markArea = markArea,
    },
    eventHandlers = {
        [MOD_NAME .. "onReceiveExteriorLocation"] = onReceiveExteriorLocation,
    },
    engineHandlers = {
        onUpdate = onUpdate,
    }
}
