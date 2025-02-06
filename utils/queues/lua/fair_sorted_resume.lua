local activeSetKey = KEYS[1]
local ownerID = ARGV[1]

local score = redis.call("ZSCORE", activeSetKey, ownerID)
if score ~= false then
    redis.call("ZADD", activeSetKey, score % 1000000, ownerID)
end
