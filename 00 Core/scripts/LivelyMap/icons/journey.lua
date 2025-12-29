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
    debug = settings.debug,
}
settings.subscribe(async:callback(function(_, key)
    settingCache[key] = settings[key]
end))


local mapUp        = false
local pathIcons    = {}

local myPaths      = nil
local minimumIndex = 1

local pathIcon     = "textures/LivelyMap/stamps/circle.png"

local baseSize     = util.vector2(16, 16)


local function attachDebugEventsToIcon(icon)
    local focusGain = function()
        local hover = {
            template = interfaces.MWUI.templates.textHeader,
            type = ui.TYPE.Text,
            alignment = ui.ALIGNMENT.End,
            props = {
                textAlignV = ui.ALIGNMENT.Center,
                relativePosition = util.vector2(0, 0.5),
                text = icon.element.layout.name .. ", path index: " .. tostring(icon.currentIdx),
            }
        }
        interfaces.LivelyMapDraw.setHoverBoxContent(hover)

        --- I think the issue is that "freed" is passed by value or something
        local registered = interfaces.LivelyMapDraw.getIcon(icon.element.layout.name)
        print(aux_util.deepToString(registered, 5))
    end

    icon.element.layout.events.focusGain = async:callback(focusGain)
    icon.element.layout.events.focusLoss = async:callback(function()
        interfaces.LivelyMapDraw.setHoverBoxContent()
        return nil
    end)
end

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
        pos = function(s)
            return s.cachedPos
        end,
        ---@param posData ViewportData
        onDraw = function(s, posData)
            -- s is this icon.
            if s.cachedPos == nil or (not posData.viewportPos.onScreen) then
                s.element.layout.props.visible = false
            else
                s.element.layout.props.size = baseSize * iutil.distanceScale(posData)
                s.element.layout.props.visible = true
                s.element.layout.props.position = posData.viewportPos.pos
            end
            s.element:update()
        end,
        onHide = function(s)
            -- s is this icon.
            --print("hiding " .. getRecord(s.entity).name)
            s.element.layout.props.visible = false
            s.element:update()
        end,
        priority = -900,
    }
    if settingCache.debug then
        attachDebugEventsToIcon(icon)
    end
    icon.element:update()
    interfaces.LivelyMapDraw.registerIcon(icon)
    return icon
end

local iconPool = pool.create(newIcon, 0)

local function color(currentIdx)
    return mutil.lerpColor(settingCache.palleteColor2, settingCache.palleteColor1,
        currentIdx / (1 + #myPaths - minimumIndex))
end

local function makeIcon(startIdx)
    local floored = math.floor(startIdx)
    local icon = iconPool:obtain()
    local name = icon.element.layout.name
    print("made journey icon at index " .. startIdx .. ". name= " .. tostring(name))
    icon.element.layout.props.visible = true
    icon.element.layout.props.color = color(floored)
    icon.currentIdx = floored
    icon.partialStep = startIdx - floored
    icon.pool = iconPool
    table.insert(pathIcons, icon)

    if settings.debug then
        local registered = interfaces.LivelyMapDraw.getIcon(name)
        print("post-register: " .. aux_util.deepToString(registered, 2))
        print(aux_util.deepToString(registered.ref.element.layout, 3))
    end
end

local function makeIcons()
    myPaths = interfaces.LivelyMapPlayerData.exteriorsOnly(
        interfaces.LivelyMapPlayerData.getPaths()
        [interfaces.LivelyMapPlayerData.playerName].paths)


    if settingCache.drawLimitNeravarinesJourney then
        local oldDuration = 4 * 60 * 60 * core.getGameTimeScale()
        local oldestTime = core.getGameTime() - oldDuration
        minimumIndex = mutil.binarySearchFirst(myPaths, function(p) return p.t > oldestTime end) or 1
        --- hard limit to 1000
        if #myPaths - minimumIndex > 1000 then
            minimumIndex = #myPaths - 1000
        end
        --- don't limit too much if we haven't moved in a long time
        if minimumIndex >= #myPaths and #myPaths > 10 then
            minimumIndex = #myPaths - 10
        end
    else
        minimumIndex = 1
    end

    print("#myPaths: " .. tostring(#myPaths) .. ", minimumIndex:" .. minimumIndex)
    if #myPaths <= 0 or minimumIndex >= #myPaths then
        return
    end

    --- TODO: BUG: If totalPips is greater than 16, some icons don't actually get rendered.
    --- They are being created and registered, though.
    --- This has something to do with the object pool pre-creating 16 objects.
    --- Lazilly-created objects are getting messed up somehow.
    --- If use 32 and toggle displaying on and off, the first and second set of icons
    --- swap being visible.
    local totalPips = 32
    local stepSize = (#myPaths - minimumIndex + 1) / totalPips

    for i = minimumIndex, #myPaths, stepSize do
        makeIcon(i)
    end
end

local function freeIcons()
    for _, icon in ipairs(pathIcons) do
        icon.element.layout.props.visible = false
        icon.cachedPos = nil
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

interfaces.LivelyMapDraw.onMapHidden(function(mapData)
    print("map down")
    if not mapData.swapped then
        print("map closed")
        mapUp = false
        displaying = false
        freeIcons()
    end
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
