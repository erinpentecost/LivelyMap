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
local interfaces = require('openmw.interfaces')
local ui         = require('openmw.ui')
local util       = require('openmw.util')
local pself      = require("openmw.self")
local types      = require("openmw.types")
local core       = require("openmw.core")
local nearby     = require("openmw.nearby")
local iutil      = require("scripts.LivelyMap.icons.iutil")
local pool       = require("scripts.LivelyMap.pool.pool")
local settings   = require("scripts.LivelyMap.settings")
local mutil      = require("scripts.LivelyMap.mutil")
local async      = require("openmw.async")
local aux_util   = require('openmw_aux.util')



local mapUp     = false
local pathIcons = {}
local myPaths   = nil


local pathIcon = "textures/LivelyMap/stamps/circle.png"

local color = util.color.rgb(223 / 255, 201 / 255, 159 / 255)
local baseSize = util.vector2(16, 16)
-- creates an unattached icon and registers it.
local function newIcon()
    local element = ui.create {
        name = "path",
        type = ui.TYPE.Image,
        props = {
            visible = false,
            position = util.vector2(100, 100),
            anchor = util.vector2(0.5, 0.5),
            size = baseSize,
            resource = ui.texture {
                path = pathIcon,
                color = color,
            }
        },
        events = {
        },
    }
    local icon = {
        element = element,
        currentIdx = nil,
        partialStep = 0,
        cachedPos = nil,
        freed = true,
        pos = function(s)
            return s.cachedPos
        end,
        ---@param posData ViewportData
        onDraw = function(s, posData)
            -- s is this icon.
            if s.freed then
                element.layout.props.visible = false
            else
                element.layout.props.size = baseSize * iutil.distanceScale(posData)
                element.layout.props.visible = true
                element.layout.props.position = posData.viewportPos.pos
            end
            element:update()
        end,
        onHide = function(s)
            -- s is this icon.
            --print("hiding " .. getRecord(s.entity).name)
            element.layout.props.visible = false
            element:update()
        end,
    }
    element:update()
    interfaces.LivelyMapDraw.registerIcon(icon)
    return icon
end

local iconPool = pool.create(function()
    return newIcon()
end)

local function makeIcon(startIdx)
    print("making journey icon at index " .. startIdx)
    local icon = iconPool:obtain()
    icon.element.layout.props.visible = true
    icon.freed = false
    icon.currentIdx = startIdx
    icon.pool = iconPool
    table.insert(pathIcons, icon)
end

local function makeIcons()
    myPaths = interfaces.LivelyMapPath.getPaths()[interfaces.LivelyMapPath.playerName].paths
    print("#myPaths: " .. tostring(#myPaths))
    if #myPaths <= 0 then
        return
    end

    for i = 1, #myPaths, 5 do
        makeIcon(i)
    end
end

local function freeIcons()
    for _, icon in ipairs(pathIcons) do
        icon.element.layout.props.visible = false
        icon.freed = true
        icon.currentIdx = nil
        icon.pool:free(icon)
    end
    pathIcons = {}
end

interfaces.LivelyMapDraw.onMapMoved(function(_)
    --- TODO: this needs to be enabled instead of always on
    print("map up")
    mapUp = true
    makeIcons()
end)

interfaces.LivelyMapDraw.onMapHidden(function(_)
    print("map down")
    mapUp = false
    freeIcons()
end)

--- how long it takes to move between two adjacent points.
local speed = 1.1

local function onUpdate(dt)
    -- Don't run if the map is not up.
    if not mapUp then
        return
    end

    -- Fake a duration if we're paused.
    if dt <= 0 then
        dt = 1 / 60
    end

    for _, icon in ipairs(pathIcons) do
        icon.partialStep = icon.partialStep + dt * speed
        local fullStep = math.floor(icon.partialStep)
        if fullStep >= 1 then
            --print("step " .. icon.currentIdx .. " done. pt: " .. icon.partialStep)
            icon.currentIdx = icon.currentIdx + fullStep
            icon.partialStep = icon.partialStep - fullStep
            if icon.currentIdx >= #myPaths then
                icon.currentIdx = 1
            end
        end
        --print("pt: " .. icon.partialStep)
        icon.cachedPos = mutil.lerpVec3(myPaths[icon.currentIdx], myPaths[icon.currentIdx + 1], icon.partialStep)
    end
end

return {
    engineHandlers = {
        onUpdate = onUpdate,
    },
}
