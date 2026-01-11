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
local interfaces   = require('openmw.interfaces')
local ui           = require('openmw.ui')
local util         = require('openmw.util')
local pself        = require("openmw.self")
local types        = require("openmw.types")
local core         = require("openmw.core")
local nearby       = require("openmw.nearby")
local iutil        = require("scripts.LivelyMap.icons.iutil")
local pool         = require("scripts.LivelyMap.pool.pool")
local settings     = require("scripts.LivelyMap.settings")
local mutil        = require("scripts.LivelyMap.mutil")
local async        = require("openmw.async")
local imageAtlas   = require('scripts.LivelyMap.h3.imageAtlas')
local aux_util     = require('openmw_aux.util')
local MOD_NAME     = require("scripts.LivelyMap.ns")

local settingCache = {
    fog = settings.main.fog,
    debug = settings.main.debug,
}
settings.main.subscribe(async:callback(function(_, key)
    settingCache[key] = settings.main[key]
end))

local TOTAL_TILES    = 30

local smokeAtlas     = imageAtlas.constructAtlas({
    totalTiles = TOTAL_TILES,
    tilesPerRow = 6,
    atlasPath = "textures/LivelyMap/smoke_atlas.png",
    tileSize = util.vector2(256, 256), -- 1279 length, 1536 width. 5 rows, 6 cols
    create = true,
})

---@type MeshAnnotatedMapData?
local currentMapData = nil

local fogIcons       = {}

-- creates an unattached icon and registers it.
local idxSeed        = 1
local function newIcon()
    idxSeed = ((idxSeed + 7) % TOTAL_TILES) + 1
    local element = smokeAtlas:spawn({
        visible = false,
        relativePosition = util.vector2(0.7, 0.7),
        anchor = util.vector2(0.5, 0.5),
        relativeSize = iutil.iconSize() * 10,
    }, idxSeed)

    local icon = {
        element = element,
        cachedPos = nil,
        pos = function(s)
            return s.cachedPos
        end,
        ---@param posData ViewportData
        onDraw = function(s, posData, parentAspectRatio)
            -- s is this icon.
            if s.cachedPos == nil or (not posData.viewportPos.onScreen) then
                s.element.layout.props.visible = false
            else
                s.element.layout.props.relativeSize = iutil.iconSize(posData, parentAspectRatio) * 10
                s.element.layout.props.visible = true
                s.element.layout.props.relativePosition = posData.viewportPos.pos
            end
            s.element:update()
        end,
        onHide = function(s)
            -- s is this icon.
            s.element.layout.props.visible = false
            s.element:update()
        end,
        priority = -5000,
    }
    icon.element:update()
    interfaces.LivelyMapDraw.registerIcon(icon)
    return icon
end

local iconPool = pool.create(newIcon, settingCache.fog and 100 or 0)

local function makeIcon(cachedPos)
    local icon = iconPool:obtain()
    icon.pool = iconPool
    icon.cachedPos = cachedPos
    table.insert(fogIcons, icon)
end

---@param extents Extents
local function makeIcons(extents, seen)
    if not settingCache.fog then
        print("no fog")
    end
    for x = extents.Left - 1, extents.Right + 1 do
        for y = extents.Bottom - 1, extents.Top + 1 do
            --print("Making for for x=" .. tostring(x) .. ", y=" .. tostring(y))
            makeIcon(mutil.cellPosToWorldPos({ x = x + .5, y = y + .5, z = 0 }))
        end
    end
end

local function freeIcons()
    for _, icon in ipairs(fogIcons) do
        icon.element.layout.props.visible = false
        icon.cachedPos = nil
        icon.currentIdx = nil
        icon.pool:free(icon)
    end
    fogIcons = {}
end

interfaces.LivelyMapToggler.onMapMoved(function(mapData)
    print("map up")
    currentMapData = mapData
    local seen = {}
    makeIcons(currentMapData.Extents, seen)
end)

interfaces.LivelyMapToggler.onMapHidden(function(mapData)
    print("map down")
    currentMapData = mapData
    freeIcons()
end)

return {}
