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
local interfaces          = require('openmw.interfaces')
local ui                  = require('openmw.ui')
local util                = require('openmw.util')
local pself               = require("openmw.self")
local types               = require("openmw.types")
local core                = require("openmw.core")
local nearby              = require("openmw.nearby")
local iutil               = require("scripts.LivelyMap.icons.iutil")
local pool                = require("scripts.LivelyMap.pool.pool")

local detectAnimalId      = core.magic.EFFECT_TYPE.DetectEnchantment
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

local animalPath = "textures/detect_animal_icon.dds"
local keyPath = "textures/detect_key_icon.dds"
local enchantmentPath = "textures/detect_enchantment_icon.dds"

local function newDetectIcon()
    return ui.create {
        name = "detect",
        type = ui.TYPE.Image,
        layer = "HUD",
        props = {
            visible = false,
            position = util.vector2(100, 100),
            anchor = util.vector2(0.5, 0.5),
            size = util.vector2(32, 32),
            resource = ui.texture {
                path = animalPath,
            }
        }
    }
end

local iconPool = pool.create(newDetectIcon)

local function makeIcon(path, entity)
    local name = getRecord(entity).name
    print("makeIcon: " .. name)
    local pip = iconPool:obtain()
    pip.layout.props.resource = ui.texture {
        path = path,
    }
    local worldPos = function()
        return entity.pos
    end
    local callbacks = {
        onDraw = function(pos)
            pip.layout.props.size = util.vector2(32, 32) * iutil.distanceScale(pos.mapWorldPos)
        end
    }
    local icon = {
        pip,
        worldPos,
        callbacks
    }
    table.insert(detectIcons, icon)
    interfaces.LivelyMapDraw.registerIcon(unpack(icon))
end

local function drawAnimals(magnitude2)
    for _, actor in ipairs(nearby.actors) do
        if types.Creature.objectIsInstance(actor) and (actor.position - pself.position):length2() <= magnitude2 then
            makeIcon(animalPath, actor)
        end
    end
end

local function drawItems(enchantmentMagnitude2, keyMagnitude2)
    for _, item in ipairs(nearby.items) do
        if getRecord(item).isKey and (item.position - pself.position):length2() <= keyMagnitude2 then
            makeIcon(keyPath, item)
        elseif getRecord(item).enchant and (item.position - pself.position):length2() <= enchantmentMagnitude2 then
            makeIcon(enchantmentPath, item)
        end
    end
    for _, container in ipairs(nearby.containers) do
        if (container.position - pself.position):length2() <= keyMagnitude2 then
            for _, item in types.Container(container) do
                if getRecord(item).isKey then
                    makeIcon(keyPath, item)
                    break
                end
            end
        end
        if (container.position - pself.position):length2() <= enchantmentMagnitude2 then
            for _, item in types.Container(container) do
                if getRecord(item) then
                    makeIcon(enchantmentPath, item)
                    break
                end
            end
        end
    end
end

local function onUpdate(dt)
    if not mapUp then
        return
    end
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
                end
            elseif effect.id == detectKeyId then
                if keyMagnitude < effect.magnitudeThisFrame then
                    keyMagnitude = effect.magnitudeThisFrame
                end
            end
        end
    end
    -- delete old icons
    if detectIcons ~= {} then
        return
    end
    for _, icon in ipairs(detectIcons) do
        iconPool:free(icon[1])
        icon[1].layout.props.visible = false
        icon[2] = nil
    end
    detectIcons = {}
    --[[print("animalMagnitude:" .. tostring(animalMagnitude) ..
        ", keyMagnitude:" .. tostring(keyMagnitude) ..
        ", enchantmentMagnitude:" .. tostring(enchantmentMagnitude)
        )]]
    -- make new icons
    animalMagnitude = 300 * 300 -- TODO: remove debug
    if animalMagnitude > 0 then
        drawAnimals(animalMagnitude * animalMagnitude)
    end
    if enchantmentMagnitude > 0 or keyMagnitude > 0 then
        drawItems(enchantmentMagnitude * enchantmentMagnitude, keyMagnitude * keyMagnitude)
    end
end

return {
    engineHandlers = {
        onUpdate = onUpdate,
    },
}
