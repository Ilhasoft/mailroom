local activeSetKey = KEYS[1]
local ownerID = ARGV[1]

-- decrement our workers for this task owner
local active = tonumber(redis.call("ZINCRBY", activeSetKey, -1, ownerID))

-- reset to zero if we somehow go below
if active < 0 then
    redis.call("ZADD", activeSetKey, 0, ownerID)
end
