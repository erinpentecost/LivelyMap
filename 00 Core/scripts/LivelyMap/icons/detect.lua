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

local detectAnimalId      = core.magic.EFFECT_TYPE.DetectEnchantment
local detectEnchantmentId = core.magic.EFFECT_TYPE.DetectEnchantment
local detectKeyId         = core.magic.EFFECT_TYPE.DetectKey

local detectIcons         = {}

local function makeIcon(tex, name, entity)
    local pip = ui.create {
        name = "detect_" .. name,
        type = ui.TYPE.Image,
        layer = "HUD",
        props = {
            visible = false,
            position = util.vector2(100, 100),
            anchor = util.vector2(0.5, 0.5),
            size = util.vector2(32, 32),
            resource = ui.texture {
                path = tex
            }
        }
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

local function getRecord(entity)
    return entity.type.records[entity.recordId]
end

local function drawAnimals(magnitude2)
    for _, actor in ipairs(nearby.actors) do
        if types.Creature.objectIsInstance(actor) and (actor.position - pself.position):length2() <= magnitude2 then
            makeIcon("textures/detect_animal_icon.dds", actor.record.name, actor)
        end
    end
end

local function drawItems(enchantmentMagnitude2, keyMagnitude2)
    for _, item in ipairs(nearby.items) do
        if getRecord(item).isKey and (item.position - pself.position):length2() <= keyMagnitude2 then
            makeIcon("textures/detect_key_icon.dds", getRecord(item).name, item)
        elseif getRecord(item).enchant and (item.position - pself.position):length2() <= enchantmentMagnitude2 then
            makeIcon("textures/detect_enchantment_icon.dds", getRecord(item).name, item)
        end
    end
    for _, container in ipairs(nearby.containers) do
        if (container.position - pself.position):length2() <= keyMagnitude2 then
            for _, item in types.Container(container) do
                if getRecord(item).isKey then
                    makeIcon("textures/detect_key_icon.dds", getRecord(item).name, item)
                    break
                end
            end
        end
        if (container.position - pself.position):length2() <= enchantmentMagnitude2 then
            for _, item in types.Container(container) do
                if getRecord(item) then
                    makeIcon("textures/detect_enchantment_icon.dds", getRecord(item).name, item)
                    break
                end
            end
        end
    end
end

local function onUpdate(dt)
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
    -- TODO: use an icon pool or something
    -- delete old icons
    for _, icon in ipairs(detectIcons) do
        icon[1]:destroy()
    end
    detectIcons = {}
    -- make new icons
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
