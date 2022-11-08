1.5.0-mailroom-7.1.44
----------
 * Merge nyaruka tag v7.1.44 into weni 1.4.5-mailroom-7.1.22 and resolve conflicts.

1.4.5-mailroom-7.1.22
----------
 * Add Ticket Fields for Zendesk #86
 * twilio flex detect and setup media on create media type  #87
 * twilio flex open ticket can set preferred flexflow from body json field flex_flow_sid #88
 * Swap targets for webhooks in Zendesk #89

1.4.4-mailroom-7.1.22
----------
 * wenichats open ticket with contact fields as default in addition to custom fields

1.4.3-mailroom-7.1.22
----------
 * fix twilio flex contact echo msgs from webhook

1.4.2-mailroom-7.1.22
----------
 * twilio flex support extra fields
 * twilio flex has Header X-Twilio-Webhook-Enabled=True on send msg

1.4.1-mailroom-7.1.22
----------
 * wenichats ticketer support custom fields

1.4.0-mailroom-7.1.22
----------
 * Add wenichats ticketer integration

1.3.3-mailroom-7.1.22
----------
 * Fix contacts msgs query

1.3.2-mailroom-7.1.22
----------
* Replace gocommon v1.16.2 with version v1.16.2-weni compatible with Teams channel

1.3.1-mailroom-7.1.22
----------
 * Replace gocommon for one with slack bot channel urn

1.3.0-mailroom-7.1.22
----------
 * Merge nyaruka tag v7.1.22 into weni 1.2.1-mailroom-7.0.1 and resolve conflicts.

1.2.1-mailroom-7.0.1
----------
 * Tweak ticketer Twilio Flex to allow API key authentication

1.2.0-mailroom-7.0.1
----------
 * Add ticketer Twilio Flex

1.1.0-mailroom-7.0.1
----------
 * Update gocommon to v1.15.1

1.0.0-mailroom-7.0.1
----------
 * Update Dockerfile to go 1.17.5
 * Fix ivr cron retry calls
 * More options in "wait for response". 15, 30 and 45 seconds
 * Support to build Docker image
