-- Modified from the original

local ui     = require("openmw.ui")
local util   = require("openmw.util")
local camera = require("openmw.camera")

---@param worldPos util.vector3
---@return boolean
local function isObjectBehindCamera(worldPos)
    -- This is from h3.
    local cameraPos = camera.getPosition()
    local cameraForward = util.transform.identity
        * util.transform.rotateZ(camera.getYaw())
        * util.vector3(0, 1, 0)

    -- Direction vector from camera to object
    local toObject = worldPos - cameraPos

    -- Normalize both vectors
    cameraForward = cameraForward:normalize()
    toObject = toObject:normalize()

    -- Calculate the dot product
    local dotProduct = cameraForward:dot(toObject)

    -- If the dot product is negative, the object is behind the camera
    return dotProduct < 0
end



---@param worldPos util.vector3
---@return util.vector2?
local function worldPosToViewportPos(worldPos)
    -- This is from h3.
    local viewportPos = camera.worldToViewportVector(worldPos)
    local screenSize = ui.screenSize()

    local validX = viewportPos.x > 0 and viewportPos.x < screenSize.x
    local validY = viewportPos.y > 0 and viewportPos.y < screenSize.y
    local withinViewDistance = viewportPos.z <= camera.getViewDistance()

    if not validX or not validY or not withinViewDistance then return end

    if isObjectBehindCamera(worldPos) then return end

    return util.vector2(viewportPos.x, viewportPos.y)
end

--- viewportPosToWorldRay builds a world-space ray from a viewport pixel.
--- This is the inverse of camera.worldToViewportVector.
---
--- @param viewportPos util.vector2 -- pixel coordinates
--- @return util.vector3? rayOrigin
--- @return util.vector3? rayDir (normalized)
local function viewportPosToWorldRay(viewportPos)
    if not viewportPos then
        error("viewportPos is nil")
    end

    local screenSize = ui.screenSize()
    if screenSize.x == 0 or screenSize.y == 0 then
        return nil, nil
    end

    -- Convert pixels → normalized screen coordinates
    local normalized = util.vector2(
        viewportPos.x / screenSize.x,
        viewportPos.y / screenSize.y
    )

    -- Camera provides a direction vector
    local rayDir = camera.viewportToWorldVector(normalized)
    if not rayDir then
        return nil, nil
    end

    return camera.getPosition(), rayDir:normalize()
end



return {
    isObjectBehindCamera = isObjectBehindCamera,
    worldPosToViewportPos = worldPosToViewportPos,
    viewportPosToWorldRay = viewportPosToWorldRay,
}
