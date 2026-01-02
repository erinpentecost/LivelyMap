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
local MOD_NAME  = require("scripts.LivelyMap.ns")
local world     = require('openmw.world')
local storage   = require('openmw.storage')
local mutil     = require("scripts.LivelyMap.mutil")

-- This file just caches cell names into global storage.
-- This makes it available to player and global scripts alike.
-- Do NOT `require` this file anywhere.

--- cell names to IDs
local cellNames = storage.globalSection(MOD_NAME .. "_cellNames")
cellNames:setLifeTime(storage.LIFE_TIME.Temporary)

local function loadCellNames()
    local tmp = {}

    -- load up with fuzzy matches
    for _, cell in ipairs(world.cells) do
        if cell.name then
            tmp[mutil.canonicalizeId(cell.name)] = cell.id
            tmp[mutil.canonicalizeId(cell.id)] = cell.id
        end
    end

    -- overwrite with exact matches
    for _, cell in ipairs(world.cells) do
        if cell.name then
            tmp[cell.name] = cell.id
        end
    end

    cellNames:reset(tmp)
end

loadCellNames()
