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

local cellData     = storage.globalSection(MOD_NAME .. "_cellNames")


local function markArea(cellName)
    local cellId = cellData:get(cellName)
    if not cellId then
        error("markArea given bad cell name: " .. tostring(cellName))
    end
    core.sendGlobalEvent(MOD_NAME .. "onGetExteriorLocation", {
        cellName = cellName,
        cellId = cellId,
        callbackObject = pself,
        source = MOD_NAME .. "_areamarker.lua",
    })
end

---@param cell any
---@param representativeObject any
local function makeTemplate(cell, representativeObject)
    -- object is basically nil or a door
    --
    -- daedric shrines usually have "shrine" in the id
    --
    return {
        iconName = "star",
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

    --- don't change the marker if it already exists
    local markId = cellId .. "_area"
    local exists = interfaces.LivelyMapMarker.getMarkerByID(markId)
    if exists then
        return
    end

    local template = makeTemplate(cellId, data.object)

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


return {
    interfaceName = MOD_NAME .. "AreaMarker",
    interface = {
        version = 1,
        markArea = markArea,
    },
    eventHandlers = {
        [MOD_NAME .. "onReceiveExteriorLocation"] = onReceiveExteriorLocation,
    },
    engineHandlers = {}
}
