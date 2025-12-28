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


local detectAnimalId      = core.magic.EFFECT_TYPE.DetectAnimal
local detectEnchantmentId = core.magic.EFFECT_TYPE.DetectEnchantment
local detectKeyId         = core.magic.EFFECT_TYPE.DetectKey

local mapUp               = false
local detectIcons         = {}

interfaces.LivelyMapDraw.onMapMoved(function(_)
    print("map up")
    mapUp = true
end)

interfaces.LivelyMapDraw.onMapHidden(function(_)
    print("map down")
    mapUp = false
end)

local function getRecord(entity)
    return entity.type.records[entity.recordId]
end

local function isEnchanted(record)
    return record.enchant ~= nil and record.enchant ~= ""
end


local animalPath = "textures/detect_animal_icon.dds"
local keyPath = "textures/detect_key_icon.dds"
local enchantmentPath = "textures/detect_enchantment_icon.dds"

local color = util.color.rgb(223 / 255, 201 / 255, 159 / 255)
local baseSize = util.vector2(32, 32)
-- creates an unattached icon and registers it.
local function newDetectIcon(path)
    local element = ui.create {
        name = "detect",
        type = ui.TYPE.Image,
        props = {
            visible = false,
            position = util.vector2(100, 100),
            anchor = util.vector2(0.5, 0.5),
            size = util.vector2(32, 32),
            resource = ui.texture {
                path = path,
            }
        },
        events = {
        },
    }
    local icon = {
        element = element,
        entity = nil,
        freed = true,
        pos = function() return nil end,
        ---@param posData ViewportData
        onDraw = function(posData, s)
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

    local focusGain = function()
        --print("focusGain: " .. aux_util.deepToString(icon.entity, 3))
        if icon.entity then
            local hover = {
                template = interfaces.MWUI.templates.textHeader,
                type = ui.TYPE.Text,
                alignment = ui.ALIGNMENT.End,
                props = {
                    textAlignV = ui.ALIGNMENT.Center,
                    relativePosition = util.vector2(0, 0.5),
                    text = getRecord(icon.entity).name,
                    textColor = color,
                }
            }
            interfaces.LivelyMapDraw.setHoverBoxContent(hover)
        end
    end

    element.layout.events.focusGain = async:callback(focusGain)
    element.layout.events.focusLoss = async:callback(function()
        --print("focusLoss: " .. aux_util.deepToString(icon.entity, 3))
        interfaces.LivelyMapDraw.setHoverBoxContent()
        return nil
    end)
    element:update()
    interfaces.LivelyMapDraw.registerIcon(icon)
    return icon
end

local iconPoolAnimal = pool.create(function()
    return newDetectIcon(animalPath)
end)
local iconPoolEnchantment = pool.create(function()
    return newDetectIcon(enchantmentPath)
end)
local iconPoolKey = pool.create(function()
    return newDetectIcon(keyPath)
end)

local function makeIcon(iconPool, entity, pos)
    --print("makeIcon: " .. getRecord(entity).name .. ", " .. tostring(pos))
    local icon = iconPool:obtain()
    icon.element.layout.props.visible = true
    icon.pos = function()
        return pos
    end
    icon.freed = false
    icon.entity = entity
    icon.pool = iconPool
    table.insert(detectIcons, icon)
end

local function magnitudeToSqDist(mag)
    if settings.extendDetectRange then
        -- 8192 at 100 mag
        local v = mag * mutil.CELL_SIZE / 100
        return v * v
    else
        -- 213 at 100 mag
        return mag * mag * 21.33333333 * 21.33333333
    end
end

local function draw(animalMagnitude, enchantmentMagnitude, keyMagnitude)
    local animalMagnitude2 = magnitudeToSqDist(animalMagnitude)
    local enchantmentMagnitude2 = magnitudeToSqDist(enchantmentMagnitude)
    local keyMagnitude2 = magnitudeToSqDist(keyMagnitude)

    -- Check for loose items first:
    for _, item in ipairs(nearby.items) do
        if getRecord(item).isKey and (item.position - pself.position):length2() <= keyMagnitude2 then
            makeIcon(iconPoolKey, item, item.position)
        elseif isEnchanted(getRecord(item)) and (item.position - pself.position):length2() <= enchantmentMagnitude2 then
            makeIcon(iconPoolEnchantment, item, item.position)
        end
    end

    -- Check actors...
    for _, actor in ipairs(nearby.actors) do
        if actor.id == pself.id then
            goto continue
        end
        -- Draw creatures.
        if types.Creature.objectIsInstance(actor) and (actor.position - pself.position):length2() <= animalMagnitude2 then
            makeIcon(iconPoolAnimal, actor, actor.position)
        end
        -- Check for items any actor carries
        if (actor.position - pself.position):length2() <= keyMagnitude2 then
            for _, item in ipairs(types.Actor.inventory(actor):getAll(types.Miscellaneous)) do
                if getRecord(item).isKey then
                    makeIcon(iconPoolKey, item, actor.position)
                    break
                end
            end
        end
        if (actor.position - pself.position):length2() <= enchantmentMagnitude2 then
            for _, item in ipairs(types.Actor.inventory(actor):getAll()) do
                if isEnchanted(getRecord(item)) then
                    makeIcon(iconPoolEnchantment, item, actor.position)
                    break
                end
            end
        end
        ::continue::
    end

    -- Check for items in containers
    for _, container in ipairs(nearby.containers) do
        if (container.position - pself.position):length2() <= keyMagnitude2 then
            for _, item in ipairs(types.Container.inventory(container):getAll(types.Miscellaneous)) do
                if getRecord(item).isKey then
                    makeIcon(iconPoolKey, item, container.position)
                    break
                end
            end
        end
        if (container.position - pself.position):length2() <= enchantmentMagnitude2 then
            for _, item in ipairs(types.Container.inventory(container):getAll()) do
                if isEnchanted(getRecord(item)) then
                    makeIcon(iconPoolEnchantment, item, container.position)
                    break
                end
            end
        end
    end
end

local function freeIcons()
    for _, icon in ipairs(detectIcons) do
        icon.pos = function()
            -- returning nil here should result
            -- in onHide() being called.
            return nil
        end
        icon.element.layout.props.visible = false
        icon.freed = true
        icon.entity = nil
        icon.pool:free(icon)
    end
    detectIcons = {}
end

local delay = -5
local function onUpdate(dt)
    -- Don't run if the map is not up.
    if not mapUp then
        -- Make sure we always run on the first frame
        -- of the map going up.
        delay = -5
        return
    end

    -- Only run about every second.
    delay = delay - dt
    if delay > 0 then
        return
    end
    delay = 1

    -- get effects we care about
    local animalMagnitude = 0
    local enchantmentMagnitude = 0
    local keyMagnitude = 0
    for _, spell in pairs(types.Actor.activeSpells(pself)) do
        for _, effect in ipairs(spell.effects) do
            if effect.magnitudeThisFrame then
                if effect.id == detectAnimalId then
                    if animalMagnitude < effect.magnitudeThisFrame then
                        animalMagnitude = effect.magnitudeThisFrame
                    end
                elseif effect.id == detectEnchantmentId then
                    if enchantmentMagnitude < effect.magnitudeThisFrame then
                        enchantmentMagnitude = effect.magnitudeThisFrame
                    end
                elseif effect.id == detectKeyId then
                    if keyMagnitude < effect.magnitudeThisFrame then
                        keyMagnitude = effect.magnitudeThisFrame
                    end
                end
            end
        end
    end
    -- DEBUG!
    --animalMagnitude = 100
    --enchantmentMagnitude = 100
    --keyMagnitude = 100
    -- delete old icons
    freeIcons()
    if animalMagnitude <= 0 and enchantmentMagnitude <= 0 and keyMagnitude <= 0 then
        return
    end
    -- make new icons
    draw(animalMagnitude, enchantmentMagnitude, keyMagnitude)
end

return {
    engineHandlers = {
        onUpdate = onUpdate,
    },
}
