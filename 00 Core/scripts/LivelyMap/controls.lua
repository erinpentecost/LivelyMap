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
local MOD_NAME        = require("scripts.LivelyMap.ns")
local mutil           = require("scripts.LivelyMap.mutil")
local putil           = require("scripts.LivelyMap.putil")
local core            = require("openmw.core")
local util            = require("openmw.util")
local pself           = require("openmw.self")
local aux_util        = require('openmw_aux.util')
local camera          = require("openmw.camera")
local ui              = require("openmw.ui")
local settings        = require("scripts.LivelyMap.settings")
local async           = require("openmw.async")
local interfaces      = require('openmw.interfaces')
local storage         = require('openmw.storage')
local input           = require('openmw.input')
local heightData      = storage.globalSection(MOD_NAME .. "_heightData")
local keytrack        = require("scripts.ErnOneStick.keytrack")
local uiInterface     = require("openmw.interfaces").UI
local h3cam           = require("scripts.LivelyMap.h3.cam")

local controls        = require('openmw.interfaces').Controls
local cameraInterface = require("openmw.interfaces").Camera

local defaultHeight   = 200
local defaultPitch    = 1

-- Exit the map when one of these triggers goes off:
for _, exitTrigger in ipairs { "Journal", "Inventory", "GameMenu" } do
    input.registerTriggerHandler(exitTrigger, async:callback(function()
        core.sendGlobalEvent(MOD_NAME .. "onHideMap", { player = pself })
    end))
end

-- Track inputs we need for navigating the map.
local keys = {
    forward  = keytrack.NewKey("forward", function(dt)
        return input.getRangeActionValue("MoveForward")
    end),
    backward = keytrack.NewKey("backward", function(dt)
        return input.getRangeActionValue("MoveBackward")
    end),
    left     = keytrack.NewKey("left", function(dt)
        return input.getRangeActionValue("MoveLeft")
    end),
    right    = keytrack.NewKey("right", function(dt)
        return input.getRangeActionValue("MoveRight")
    end)
}

-- Reset inputs so they don't get stuck.
local function clearControls()
    pself.controls.sideMovement = 0
    pself.controls.movement = 0
    pself.controls.yawChange = 0
    pself.controls.pitchChange = 0
    pself.controls.run = false
end

---@class CameraData
---@field pitch number?
---@field yaw number?
---@field roll number?
---@field position util.vector3?
---@field relativePosition util.vector3?
---@field mode any

---@type MeshAnnotatedMapData?
local currentMapData = nil

---@return CameraData
local function currentCameraData()
    return {
        pitch = camera.getPitch(),
        yaw = camera.getYaw(),
        position = camera.getPosition(),
        roll = camera.getRoll(),
        mode = camera.getMode()
    }
end

---@param data CameraData
local function setCamera(data)
    camera.setMode(data.mode, true)
    camera.setPitch(data.pitch)
    camera.setYaw(data.yaw)
    camera.setRoll(data.roll)
end

-- Store old camera state so we can reset to that same state
-- once we exit the map.
-- Then set the initial camera state we need.
---@type CameraData?
local originalCameraState = nil
local function startCamera()
    controls.overrideMovementControls(true)
    cameraInterface.disableModeControl(MOD_NAME)
    controls.overrideUiControls(true)
    uiInterface.setHudVisibility(false)
    clearControls()
    if originalCameraState == nil then
        -- Don't override the old state.
        -- this might be called multiple times before
        -- endCamera() is called.
        originalCameraState = currentCameraData()
    end
    camera.setMode(camera.MODE.Static, true)
end

--- Restore the camera back to original state.
local function endCamera()
    controls.overrideMovementControls(false)
    cameraInterface.enableModeControl(MOD_NAME)
    controls.overrideUiControls(false)
    uiInterface.setHudVisibility(true)
    clearControls()
    if originalCameraState then
        setCamera(originalCameraState)
    end
    originalCameraState = nil
end

local function facing2D(camViewVector)
    local viewDir = camViewVector or camera.viewportToWorldVector(util.vector2(0.5, 0.5))
    return util.vector3(viewDir.x, viewDir.y, 0):normalize()
end


---@class ScreenHit
---@field hitMap boolean Whether this corner collided with the map mesh or not.
---@field normalizedScreenPosition util.vector2 The normalized screen position for this corner.
---@field worldSpace util.vector3? The world space coordinate where the ray collided with a plane that is coplanar to the map mesh.

---@class ScreenHits
---@field bottomLeft ScreenHit
---@field bottomRight ScreenHit
---@field topLeft ScreenHit
---@field topRight ScreenHit

---@param data ScreenHits
---@return number
local function screenPositionsValid(data)
    local count = 4
    for _, pos in pairs(data) do
        if not pos.hitMap then
            count = count - 1
        end
    end
    return count
end

---@return ScreenHits
local function getScreenPositions()
    ---@type ScreenHits
    local out = {
        topLeft = {
            hitMap = false,
            normalizedScreenPosition = util.vector2(0, 0),
        },
        topRight = {
            hitMap = false,
            normalizedScreenPosition = util.vector2(1, 0),
        },
        bottomLeft = {
            hitMap = false,
            normalizedScreenPosition = util.vector2(0, 1),
        },
        bottomRight = {
            hitMap = false,
            normalizedScreenPosition = util.vector2(1, 1),
        },
    }


    local bounds = currentMapData ~= nil and currentMapData.safeBounds
    if not bounds then
        return out
    end

    -- Extract rectangle extents
    local minX = bounds.bottomLeft.x
    local maxX = bounds.bottomRight.x
    local minY = bounds.bottomLeft.y
    local maxY = bounds.topLeft.y
    local planeZ = bounds.bottomLeft.z

    --print("bounds: " .. aux_util.deepToString(bounds, 3))

    local camPos = camera.getPosition()

    -- Camera must be above the map plane
    if camPos.z <= planeZ then
        return out
    end

    for k, screenPos in pairs(out) do
        local dir = camera.viewportToWorldVector(screenPos.normalizedScreenPosition):normalize()

        -- Ray must intersect the plane
        if math.abs(dir.z) < 1e-6 then
            --[[print("no intersection for screenPos " ..
                aux_util.deepToString(screenPos, 3) ..
                ", dir: " .. tostring(dir) .. ", camerapos: " .. tostring(camera.getPosition()))]]
            goto continue
        end

        local t = (planeZ - camPos.z) / dir.z

        -- Intersection must be in front of camera
        if t <= 0 then
            --[[print("intersection behind camera for screenPos " ..
                aux_util.deepToString(screenPos, 3) ..
                ", dir: " .. tostring(dir) .. ", camerapos: " .. tostring(camera.getPosition()) ..
                ", t: " .. tostring(t))]]
            goto continue
        end

        out[k].worldSpace = camPos + dir * t

        -- 2D bounds containment
        if out[k].worldSpace.x < minX or out[k].worldSpace.x > maxX
            or out[k].worldSpace.y < minY or out[k].worldSpace.y > maxY then
            --[[print("intersection beyond bounds for screenpos " ..
                aux_util.deepToString(screenPos, 3) ..
                ", dir: " ..
                tostring(dir) ..
                ", camerapos: " ..
                tostring(camera.getPosition()) .. ", t: " .. tostring(t) .. ", hit:" .. tostring(out[k].worldSpace))]]
            goto continue
        end

        -- this point is in the mesh
        out[k].hitMap = true
        :: continue ::
    end

    return out
end

--- moveCamera safely moves the camera within acceptable bounds.
--- Once we move the camera, we won't be able to reliable read
--- from it for the rest of the frame.
---@param data CameraData?
local function moveCamera(data)
    if data == nil then
        return
    end

    local currentPosition = camera.getPosition()
    --- newPos replaces data.position or data.relativePosition.
    --- This is done because we might specify a relative position
    --- instead of an absolute position for the camera.
    ---@type util.vector3
    local newPos = currentPosition
    if data.position or data.relativePosition then
        newPos = data.position or (currentPosition + data.relativePosition)
    end

    local screenPositions = getScreenPositions()
    local validPositions = screenPositionsValid(screenPositions)

    if validPositions ~= 4 then
        print("Map collision failure.")
        if (not screenPositions.topLeft.hitMap) and (not screenPositions.topRight.hitMap) then
            -- we are too far up.
            if currentPosition.y <= newPos.y then
                newPos = util.vector3(newPos.x, currentPosition.y, newPos.z)
            end
        end
        if (not screenPositions.bottomLeft.hitMap) and (not screenPositions.bottomRight.hitMap) then
            -- we are too far down.
            if currentPosition.y >= newPos.y then
                newPos = util.vector3(newPos.x, currentPosition.y, newPos.z)
            end
        end
        if (not screenPositions.topLeft.hitMap) and (not screenPositions.bottomLeft.hitMap) then
            -- we are too far to the left
            if currentPosition.x >= newPos.x then
                newPos = util.vector3(currentPosition.x, newPos.y, newPos.z)
            end
        end
        if (not screenPositions.topRight.hitMap) and (not screenPositions.bottomRight.hitMap) then
            -- we are too far to the right
            if currentPosition.x <= newPos.x then
                newPos = util.vector3(currentPosition.x, newPos.y, newPos.z)
            end
        end
    end


    if data.pitch then
        camera.setPitch(data.pitch)
    end
    if data.yaw then
        camera.setYaw(data.yaw)
    end
    if newPos then
        camera.setStaticPosition(newPos)
    end
end


---@class TrackInfo
---@field tracking boolean
---@field startTime number
---@field endTime number
---@field onEnd fun(finished: boolean)?
---@field startCameraData CameraData?
---@field endCameraData CameraData?

--- Lerp the camera to a new position.
---@type TrackInfo
local trackInfo = {
    tracking = false,
    startTime = 0,
    endTime = 0,
    onEnd = nil,
    startCameraData = nil,
    endCameraData = nil,
}

local function advanceTracker()
    if not trackInfo.tracking then
        return
    end
    local currentTime = core.getRealTime()
    local intermediate = nil
    local i = 0
    if currentTime >= trackInfo.endTime then
        -- set to end
        i = 1
        intermediate = {
            position = trackInfo.endCameraData.position,
            pitch = trackInfo.endCameraData.pitch,
        }
    else
        -- lerp!
        i = util.remap(currentTime, trackInfo.startTime, trackInfo.endTime, 0, 1)
        intermediate = {
            position = mutil.lerpVec3(trackInfo.startCameraData.position, trackInfo.endCameraData.position, i),
            pitch = mutil.lerpAngle(trackInfo.startCameraData.pitch, trackInfo.endCameraData.pitch, i)
        }
    end

    if i >= 1 then
        trackInfo.tracking = false
        if trackInfo.onEnd then
            trackInfo.onEnd(true)
        end
    end
    --print("advanceTracker: " .. i .. ": " .. aux_util.deepToString(intermediate, 3))
    -- TODO: maybe do fancy camera movements in the future
    moveCamera(intermediate)
end

---@param cameraData CameraData
local function trackPosition(cameraData, duration, onEnd)
    if cameraData == nil then
        error("trackPosition cameraData is required.")
    end
    trackInfo.tracking = true
    trackInfo.startCameraData = currentCameraData()
    trackInfo.endCameraData = cameraData

    if trackInfo.endCameraData.relativePosition and not trackInfo.endCameraData.position then
        trackInfo.endCameraData.position = camera.getPosition() + trackInfo.endCameraData.relativePosition
        trackInfo.endCameraData.relativePosition = nil
    end

    trackInfo.startTime = core.getRealTime()
    duration = duration or 0
    trackInfo.endTime = trackInfo.startTime + duration
    trackInfo.onEnd = onEnd

    print("trackPosition: " .. aux_util.deepToString(cameraData, 3))

    -- Immediate advance if no duration given.
    if duration <= 0 then
        advanceTracker()
    end
end

-- cameraOffset returns a vector offset for the camera position
-- so that the center of the viewPort lands on targetPosition.
local function cameraOffset(targetPosition, camPitch, camViewVector)
    local pos = targetPosition or camera.getPosition()
    local pitch = camPitch or camera.getPitch()
    local height = pos.z - currentMapData.object.position.z
    local viewDir = facing2D(camViewVector)
    print(viewDir)
    -- 1.5708 - pitch is the angle between straight down and camera center.
    return viewDir * (-1 * height * math.tan(1.5708 - pitch))
end

local function trackToWorldPosition(worldPos, duration, onEnd)
    camera.setYaw(0)
    local mapCenter = currentMapData.object:getBoundingBox().center
    local cellPos = mutil.worldPosToCellPos(worldPos)
    local rel = putil.cellPosToRelativeMeshPos(currentMapData, cellPos)
    local mapWorldPos = putil.relativeMeshPosToAbsoluteMeshPos(currentMapData, rel)
    local heightOffset = util.vector3(0, 0, defaultHeight)
    local camOffset = cameraOffset(mapCenter + heightOffset, defaultPitch, util.vector3(0, 1, 0))
    trackPosition({
        pitch = defaultPitch,
        position = mapWorldPos + camOffset + heightOffset,
    }, duration, onEnd)
end

local vecForward = util.vector3(0, 1, 0)
local vecBackward = vecForward * -1
local vecRight = util.vector3(1, 0, 0)
local vecLeft = vecRight * -1
local moveSpeed = 100

local function onFrame(dt)
    -- Fake a duration if we're paused.
    if dt <= 0 then
        dt = 1 / 60
    end
    -- Only track inputs while the map is up.
    if not originalCameraState then
        return
    end
    -- Track inputs.
    keys.forward:update(dt)
    keys.backward:update(dt)
    keys.left:update(dt)
    keys.right:update(dt)
    -- If we have input, cancel trackPosition,
    -- then move the camera manually.
    -- Else, advance the camera toward tracked position.
    local hasInput = keys.forward.pressed or keys.backward.pressed or keys.left.pressed or keys.right.pressed
    if hasInput then
        local moveVec = (vecForward * keys.forward.analog +
            vecBackward * keys.backward.analog +
            vecRight * keys.right.analog +
            vecLeft * keys.left.analog):normalize() * moveSpeed * dt

        moveCamera({
            relativePosition = moveVec
        })
        -- Interrupt tracking
        if trackInfo.tracking then
            if trackInfo.onEnd then
                trackInfo.onEnd(false)
            end
        end
        trackInfo.tracking = false
    else
        advanceTracker()
    end
end


local function onMapMoved(data)
    print("controls.onMapMoved")
    currentMapData = data
    -- If this is not a swap, then this is a brand new map session.
    if not data.swapped then
        -- Orient the camera so starting position is in the center.
        startCamera()
        trackToWorldPosition(data.startWorldPosition)
    end
end

local function onMapHidden(data)
    print("controls.onMapHidden")
    currentMapData = nil
    -- If it's not a swap, it means we are done looking at the map.
    if not data.swapped then
        endCamera()
    end
end

return {
    interfaceName = MOD_NAME .. "Controls",
    interface = {
        version = 1,
        trackPosition = trackPosition,
        trackToWorldPosition = trackToWorldPosition,
    },
    engineHandlers = {
        onFrame = onFrame,
    },
    eventHandlers = {
        [MOD_NAME .. "onMapMoved"] = onMapMoved,
        [MOD_NAME .. "onMapHidden"] = onMapHidden,
    },
}
