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
local aux_util     = require('openmw_aux.util')
local MOD_NAME     = require("scripts.LivelyMap.ns")

local settingCache = {
    palleteColor1 = settings.palleteColor1,
    palleteColor2 = settings.palleteColor2,
    drawLimitNeravarinesJourney = settings.drawLimitNeravarinesJourney,
}
settings.subscribe(async:callback(function(_, key)
    settingCache[key] = settings[key]
end))

settings.subscribe(async:callback(function(_, key)
    settingCache[key] = settings[key]
end))


local mapUp        = false
local pathIcons    = {}

local myPaths      = nil
local minimumIndex = 1

local pathIcon     = "textures/LivelyMap/stamps/circle.png"

local baseSize     = util.vector2(16, 16)
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
            color = settingCache.palleteColor2,
            resource = ui.texture {
                path = pathIcon,
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

local function color(currentIdx)
    return mutil.lerpColor(settingCache.palleteColor2, settingCache.palleteColor1,
        currentIdx / (1 + #myPaths - minimumIndex))
end

local function makeIcon(startIdx)
    ---print("making journey icon at index " .. startIdx)
    local icon = iconPool:obtain()
    icon.element.layout.props.visible = true
    icon.element.layout.props.color = color(startIdx)
    icon.freed = false
    icon.currentIdx = startIdx
    icon.pool = iconPool
    table.insert(pathIcons, icon)
end


---@param paths PathEntry[]  -- sorted by increasing t
---@param oldestTime number
---@return integer? index    -- nil if not found
local function findOldestAfter(paths, oldestTime)
    local lo = 1
    local hi = #paths
    local result = nil

    while lo <= hi do
        local mid = math.floor((lo + hi) / 2)
        local t = paths[mid].t

        if t > oldestTime then
            result = mid -- candidate; try to find an earlier one
            hi = mid - 1
        else
            lo = mid + 1
        end
    end
    return result
end

local function makeIcons()
    myPaths = interfaces.LivelyMapPlayerData.getPaths()[interfaces.LivelyMapPlayerData.playerName].paths

    if settingCache.drawLimitNeravarinesJourney then
        local oldDuration = 4 * 60 * 60 * core.getGameTimeScale()
        local oldestTime = core.getGameTime() - oldDuration
        minimumIndex = findOldestAfter(myPaths, oldestTime) or 1
        if #myPaths - minimumIndex > 1000 then
            minimumIndex = #myPaths - 1000
        end
    else
        minimumIndex = 1
    end

    print("#myPaths: " .. tostring(#myPaths) .. ", minimumIndex:" .. minimumIndex)
    if #myPaths <= 0 or minimumIndex > #myPaths then
        return
    end

    for i = minimumIndex, #myPaths, 5 do
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

local displaying = false

interfaces.LivelyMapDraw.onMapMoved(function(mapData)
    print("map up")
    mapUp = true
end)

interfaces.LivelyMapDraw.onMapHidden(function(_)
    print("map down")
    mapUp = false
end)



--- how long it takes to move between two adjacent points.
local speed = 1.1

local function onUpdate(dt)
    -- Don't run if the map is not up.
    if not mapUp then
        return
    end

    if not displaying then
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
        end
        if icon.currentIdx >= #myPaths then
            icon.currentIdx = minimumIndex
        end
        --print("pt: " .. icon.partialStep)
        icon.cachedPos = mutil.lerpVec3(myPaths[icon.currentIdx], myPaths[icon.currentIdx + 1], icon.partialStep)
        icon.element.layout.props.color = color(icon.currentIdx)
    end
end

return {
    interfaceName = MOD_NAME .. "JourneyIcons",
    interface = {
        version = 1,
        toggleJourney = function()
            if displaying then
                freeIcons()
            else
                makeIcons()
            end
            displaying = not displaying
        end,
    },
    engineHandlers = {
        onUpdate = onUpdate,
    },
}
