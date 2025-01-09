local activeSetKey = KEYS[1]
local queueBase = ARGV[1]

-- first get what is the active queue
local result = redis.call("ZRANGE", activeSetKey, 0, 0)

-- nothing? return nothing
local ownerID = result[1]
if not ownerID then
    return {"empty", ""}
end

local queueKey = queueBase .. ":" .. ownerID

-- pop off our queue
local result = redis.call("ZPOPMIN", queueKey)

-- found a result?
if result[1] then
    -- and add a worker to this owner
    redis.call("ZINCRBY", activeSetKey, 1, ownerID)

    return {ownerID, result[1]}
else
    -- no result found, remove this owner from active queues
    redis.call("ZREM", activeSetKey, ownerID)

    return {"retry", ""}
end
