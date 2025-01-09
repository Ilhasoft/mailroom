local activeSetKey = KEYS[1]
local queueBase = ARGV[1]

local results = redis.call("ZRANGE", activeSetKey, 0, -1)
local count = 0

for i = 1, #results do
    local result = redis.call("ZCARD", queueBase .. ":" .. results[i])
    count = count + result
end

return count