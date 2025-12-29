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
local storage = require('openmw.storage')

-- This file just loads the JSON map data into global storage.
-- This makes it available to player and global scripts alike.
-- Do NOT `require` this file anywhere.

local markerData = storage.playerSection(MOD_NAME .. "_markerData")
markerData:setLifeTime(storage.LIFE_TIME.Persistent)

---@class MarkerData
---@field id string Unique internal ID.
---@field hidden boolean Basically soft-delete.
---@field worldPos util.vector3
---@field iconName string Base filename, including extension.
---@field manual boolean True if the user made it, as opposed to automatic.
---@field note string? This appears in the hover box.

---@param id string
---@return MarkerData?
local function getMarkerByID(id)
    if not id or (type(id) ~= "string") then
        error("getMarkerByID(): id is bad")
        return
    end
    return markerData:get(id)
end

---@return {[string]: MarkerData}
local function getAllMarkers()
    return markerData:asTable()
end

---Makes a new marker if it does not exist.
---@param data MarkerData
local function newMarker(data)
    if not data or not data.id or not type(data.id) == "string" then
        error("newMarker: bad data")
    end
    if not getMarkerByID(data.id) then
        markerData:set(data.id, data)
    end
end

---@param data MarkerData
local function upsertMarker(data)
    if not data or not data.id or not type(data.id) == "string" then
        error("newMarker: bad data")
    end
    markerData:set(data.id, data)
end



return {
    interfaceName = MOD_NAME .. "Marker",
    interface = {
        version = 1,
        getMarkerByID = getMarkerByID,
        newMarker = newMarker,
        upsertMarker = upsertMarker,
    },
}
