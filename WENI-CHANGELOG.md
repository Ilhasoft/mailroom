1.96.1
----------
* feat: log when a ticket as closed by merge on zendesk ticketers

1.96.0
----------
* feat: depend on goflow whatsapp_bsuid and whatsapp_phone contact query attributes (goflow v1.20.4)

1.95.0
----------
* feat: add whatsapp_username, instagram_username and ctwa_clid field with auto-creation and validation in tests

1.94.1
----------

* feat: implementing contact history send for generic ticketer, include default payload and optional template for request/response

* doc: add doc for generic ticketer integrations

1.94.0
----------
* feat: add optional request/response templates to generic ticketer (open, forward, close and inbound messages/tickets/close)

1.93.0
----------
* feat: add marketing opt-in functionality with tests and database integration

1.92.0
----------
* feat: add generic ticketer

1.91.0
----------
* feat: add product carousel support to WppBroadcastMessage and related tests

1.90.0
----------
* feat: add vtex_account field support in contact field management

1.89.0
----------
* feat: add Zendesk Marketplace and OAuth configuration support

1.88.0
----------
* fix: stop wiping ticket external_id on change_ticketer; accept optional external_id and preserve the existing one when not provided
* chore: Update go.mod and go.sum to replace gocommon version with v1.17.0

1.87.1
----------
* feat: enhance WppBroadcastMessage to include DirectSendTemplateName

1.87.0
----------
* feat: add session field support in contact field modifiers

1.86.0
----------
* feat: add POST /mr/ticket/change_ticketer endpoint to move tickets between ticketers without closing them

1.85.4
----------
* feat: add email field support in contact field modifiers

1.85.3
----------
* feat: enable stream support for channel versions >= 2

1.85.2
----------
* feat: check if is ab2(is_multi_agents) before publish sqs ticket event

1.85.1
----------
 * chore: update goflow for v1.20.2

1.85.0
----------
 * feat: implement GetOrCreateContactField function and add tests for contact field creation and modifiers

1.84.0
----------
 * feat: update goflow dependency to version v1.20.1 and add flow_token to outgoing message structure

1.83.0
----------
 * chore: update goflow dependency to version v1.20.0 and add direct_send and ttl_seconds fields to WppBroadcastMessage

1.82.0
----------
 * chore: update goflow dependency to version v1.19.1

1.81.1
----------
 * feat: refactor handleMsgEvent

1.81.0
----------
 * feat: Add CarouselMessage support in WppBroadcastMessage

1.80.1
----------
 * chore: update goflow 1.18.2

1.80.0
----------
 * feat: sqs worker, producer and publish ticket events

1.79.0
----------
 * feat: persist queue name on msg metadata on create msg

1.78.1
----------
 * chore: update goflow 1.18.1

1.78.0
----------
 * chore: Update go.mod and go.sum to replace the goflow version with v1.18.0 and adjust the default broadcast message construction

1.77.2
----------
 * feat: repair session with empty error and refactor open ticket wenichats to avoid return empty error

1.77.1
----------
 * feat: send stream support parameter on request to router

1.77.0
----------
 * feat: Enhance WppBroadcast functionality with carousel support and template evaluation

1.76.1
----------
 * Introduce a fixed token for wenichats authentication and remove the keycloak's one

1.76.0
----------
 * Handle new contact fields on MsgEvent
 * Add ChannelType to requestToRouter struct

1.75.1
----------
* Concatenating contact name with identity on twilio flex 2 so we can se the participant name on his identity

1.75.0
----------
 * Log pending tasks on exit

1.74.0
----------
 * Task send_history has status processing for retry task if the instance closes abruptly before completion

1.73.5
----------
 * Now wenichats send default history as recent 24 hours msgs

1.73.4
----------
 * Fix dynamic groups calculation for message events if brain on

1.73.3
----------
 * Add source field to Channel struct and update tests

1.73.2
----------
 * Fix twilioflex2 interaction customer participant client parameter
 * Fix Twilioflex2 media response parameter 

1.73.1
----------
 * Twilioflex2 history media support
 * Twilioflex2 dynamic channel type between whatsapp and web
 * Add Twilioflex2 interaction customer participant to enable pause

1.73.0
----------
 * Update goflow version to v1.16.0

1.72.0
---------- 
 * Ticket open rabbitmq event

1.71.0
----------
 * Add Freshchat client and service implementation

1.70.2
----------
 * Fix Twilio Flex 2 receive media

1.70.1
----------
 * Twilio Flex 2 interaction task attributes handling

1.70.0
----------
 * Add Twilio Flex for flex UI 2.x integration

1.69.1
----------
 * Fix whatsapp URN variation generation to handle only 13 and 12 digit numbers

1.69.0
----------
 * Update goflow version to v1.15.0
 * Refactor contact loading to use LoadContacts method across multiple handlers

1.68.0
----------
 * Add is_bulk_send field to broadcast table
 * Use created_on as a check field in triggers

1.67.0
----------
 * Update goflow version to v1.14.1

1.66.0
----------
 * Add WPP broadcast and template batch support to Mailroom in specific queues

1.65.4
----------
 * Retry for ticketer send history task

1.65.3
----------
 * Add contact name atribute to msg struct

1.65.2
----------
 * Tweak select eventfire query with explicit joins to avoid implicit less performatic joins
 * Optimize contact loading with LoadContactsBasic

1.65.1
----------
 * Add protocol on request to open wenichats room

1.65.0
----------
 * Enhance requestToRouter function to include channel information and contact name.

1.64.0
----------
 * Add category field to Template model and update SQL selection

1.63.1
----------
 * Fix to suspend template org config for Check suspend_template on newOutgoingMsg too

1.63.0
----------
 * SendHistory ticketer task

1.62.0
----------
 * SendHistory ticketer actions

1.61.2
----------
 * Fix open ticket error message return

1.61.1
----------
 * Add condition for linx API type in cart simulation logic

1.61.0
----------
 * Enhance WppBroadcastMessage structure by adding ActionType and ActionExternalID fields.

1.60.2
----------
 * Fail template msg if org conf suspend_template is true

1.60.1
----------
 * feat: Set default high priority for outgoing WPP messages
 * remove return on worker handleMsgEvent on isBrain

1.60.0
----------
 * Add Linx Search functionality to retrieve product

1.59.0
----------
 * Add OIDC configuration and enhance Wenichats client with token management

1.58.2
----------
 * Fix hideUnavailable implementation

1.58.1
----------
 * Fix open ticket endpoint putting back the time parse format for "2006-01-02 15:04:05-07:00" on wenichats service parseTime

1.58.0
----------
 * Send external_id field to wenichats when creating message
 * propagate wenichats msg UUID through msg metadata

1.57.0
----------
 * Modify service logic to include search keywords in call results
 * Add logistics info and delivery channel filter to seller search

1.56.0
----------
 * feat: Wenichats room with history
 * feat: send msg metadata with context to wenichats

1.55.0
----------
 * Add support for Instagram replies: comments, by tag and private reply

1.54.0
----------
 * feat: support url parameters for cart simulation

1.53.0
----------
 * Add locale field to templates in broadcast messages

1.52.0
----------
 * twillio-flex use the match of contactID and ticketUUID as identity for user

1.51.0
----------
 * Add support for catalog message in whatsapp messages

1.50.3
----------
 * Update go version to 1.23 and dockerfile go and alpine versions

1.50.2
----------
 * Remove log "fcm not configured"

1.50.1
----------
 * Fix worker metrics

1.50.0
----------
 * Send contact fields to router

1.49.3
----------
 * Fix http_logs insert url size error, truncating if greater than 2048

1.49.2
----------
 * Fix template field to save null when empty

1.49.1
----------
 * Send created_on on wenichats message forward

1.49.0
----------
 * Use extra prompt in search parameters

1.48.1
----------
 * Temporarily remove Zendesk user search and creation

1.48.0
----------
 * Add handle for zendesk csat

1.47.5
----------
 * Add tolerance time to wait before shutdown mr webserver to avoid 502 and 503 errors

1.47.4
----------
 * Sending the ticket uuid when opening wenichats rooms #211

1.47.3
----------
 * Fix session ended_on null cases
 * Add readme for mailroom services
 * Increase test coverage for product research and bug fixes

1.47.2
----------
 * Fix send_wpp_broadcast for msgs with failed reason D

1.47.1
----------
 * Remove # at the beginning of sellerID

1.47.0
----------
 * Revert "Merge users zendesk"
 * Revert "Search for Zendesk user with identities and add retry"

1.46.1
----------
 * Remove seller 1

1.46.0
----------
 * Use sellerId returned in search in specific cases

1.45.1
----------
 * Fix: Ninth digit generated owner urn map for broadcasts

1.45.0
----------
 * Search for Zendesk user with identities and add retry

1.44.0
----------
 * Merge users zendesk

1.43.0
----------
 * feat: add buttons support to wpp broadcasts
 * feat: add channel option to whatsapp broadcasts

1.42.0
----------
 * Fix: handle new broadcast_type field
 * Refactor product search

1.41.0
----------
 * Feat: WhatsApp Broadcasts

1.40.4
----------
 * Update goflow to v1.5.2

1.40.3
----------
 * Undo refactoring for Meta search

1.40.2
----------
 * Add topic queue uuid 

1.40.1
----------
 * Refactor meta search

1.40.0
----------
 * Use hideUnavailable for product search

1.39.2
----------
 * Update wenichats service try close room on fail in any step to open ticket
 * Fix URN Brain
 * Update goflow to v1.4.2

1.39.1
----------
 * Update goflow to v1.4.1

1.39.0
----------
 * Feat: WhatsApp Order Details

1.38.0
----------
 * update goflow to v1.3.0

1.37.0
----------
 * update goflow to v1.2.1

1.36.0
----------
 * update goflow to v1.2.0-a1

1.35.0
----------
 * Refactor vtex ads
 * Feat: Add support to WhatsApp Flows

1.34.6-mailroom-7.1.22
----------
 * Fix brain to only send attachments when entry is "@input.text"

1.34.5-mailroom-7.1.22
----------
 * Fix open wenichats on open ticket to handle properly for contact without preferred urn

1.34.4-mailroom-7.1.22
----------
 * Fix vtex intelligent search url

1.34.3-mailroom-7.1.22
----------
 * Allow locale query param on vtex intelligent search
 * Update goflow for v0.14.2-goflow-0.144.3

1.34.2-mailroom-7.1.22
----------
 * Update goflow for v0.14.1-goflow-0.144.3

1.34.1-mailroom-7.1.22
----------
 * Handle invalid vtex api search type

1.34.0-mailroom-7.1.22
----------
 * Handle brain flowstart msg event with order

1.33.1-mailroom-7.1.22
----------
 * Return call result if cart simulation fails

1.33.0-mailroom-7.1.22
----------
 * Update goflow to v0.14.0-goflow-0.144.3

1.32.0-mailroom-7.1.22
----------
 * Add ninth digit verification to allowed orgs

1.31.1-mailroom-7.1.22
----------
 * Update goflow for v0.13.1-goflow-0.144.3

1.31.0-mailroom-7.1.22
----------
 * Update goflow for v0.13.0-goflow-0.144.3

1.30.1-mailroom-7.1.22
----------
 * Ensure ticket close even if wenichats close room fails

1.30.0-mailroom-7.1.22
----------
 * Support for CTA message for whatsapp cloud channels

1.29.0-mailroom-7.1.22
----------
 * Add sponsored search in the send products card

1.28.1-mailroom-7.1.22
----------
 * Remove weni insights integration

1.28.0-mailroom-7.1.22
----------
 * Popular template column in msgs_msg table

1.27.2-mailroom-7.1.22
----------
 * Remove empty sessions

1.27.1-mailroom-7.1.22
----------
 * Remove return_only_approved_products field

1.27.0-mailroom-7.1.22
----------
 * Add product search to Meta to target availability

1.26.2-mailroom-7.1.22
----------
 * Update goflow version to v0.11.2-goflow-0.144.3 with correct release

1.26.1-mailroom-7.1.22
----------
 * Update goflow version to v0.11.1-goflow-0.144.3 for correct brain webhook call

1.26.0-mailroom-7.1.22
----------
 * Add action to brain card

1.25.0-mailroom-7.1.22
----------
 * Add Metrics by org and globally to prometheus
 * Fix Load Catalogs

1.24.0-mailroom-7.1.22
----------
 * Implementations for whatsapp message sending card
   
1.23.1-mailroom-7.1.22
----------
 * Update goflow version to v0.9.1-goflow-0.144.3 for custom webhooks timeouts

1.23.0-mailroom-7.1.22
----------
 * Add insights integration and send flowrun data to it on create or update

1.22.0-mailroom-7.1.22
----------
 * Add WhatsApp token header on callWebhook actions

1.21.0-mailroom-7.1.22
----------
 * New flow batch queue
 * Send direct message endpoint
 * Add workers prometheus metrics

1.20.1-mailroom-7.1.22
----------
 * Update goflow version to v0.8.1-goflow-0.144.3

1.20.0-mailroom-7.1.22
----------
 * Add brain_on field to Org and send messages to the Router

1.19.2-mailroom-7.1.22
----------
 * add env for flow start batch size

1.19.1-mailroom-7.1.22
----------
 * close wenichats ticket on history failure

1.19.0-mailroom-7.1.22
----------
 * Fix simulator max value length
 * Wenichats history_after on body param

1.18.0-mailroom-7.1.22
----------
 * Adjust chatgpt prompt for product list
 * Perform cart simulation for product list using postal code

1.17.3-mailroom-7.1.22
----------
 * /mr/health do health check for redis, database, sentry and s3

1.17.2-mailroom-7.1.22
----------
 * handling tickets/utils SendReply msg return
 * twilioflex open ticket handling to close cleanup flex resource if webhook conf fail

1.17.1-mailroom-7.1.22
----------
 * Improve prompt for chatGPT in product listings

1.17.0-mailroom-7.1.22
----------
 * Fix errors for duplicate products

1.16.0-mailroom-7.1.22
----------
 * Implement changes to preserve the insertion order in the product list

1.15.0-mailroom-7.1.22
----------
 * Update goflow version to v0.6.3-goflow-0.144.3

1.14.0-mailroom-7.1.22
----------
 * Update goflow version to v0.6.2-goflow-0.144.3

1.13.1-mailroom-7.1.22
----------
 * fix wenichats sending history when opening ticket, adding a margin time in the selection parameter to avoid omitting the first message that was created before the flowrun

1.13.0-mailroom-7.1.22
----------
 * Implement the nfm_reply field in input

1.12.0-mailroom-7.1.22
----------
 * Add handle for weniGPTCalled type events
 * Revert "Merge pull request #70 from weni-ai/fix/wenichats-send-history"

1.11.2-mailroom-7.1.22
----------
 * Fix sellerId logic for intelligent vtex search request

1.11.1-mailroom-7.1.22
----------
 * fix wenichats send history on open ticket based on first message and flowrun events

1.11.0-mailroom-7.1.22
----------
 * Update goflow version to v0.5.2-goflow-0.144.3

1.10.0-mailroom-7.1.22
----------
 * Update goflow version to v0.5.1-goflow-0.144.3

1.9.1-mailroom-7.1.22
----------
 * Change org config data name

1.9.0-mailroom-7.1.22
----------
 * Add orgContext asset

1.8.0-mailroom-7.1.22
----------
 * Vtex search support with sellerId
 * [FLOWS-285] - Add contact in httplog model

1.7.3-mailroom-7.1.22
----------
 * Add support for searching for Vtex products

1.7.2-mailroom-7.1.22
----------
 * Remove duplicate products for catalog messages

1.7.1-mailroom-7.1.22
----------
 * Tweaks for close ticket behaviour to not delete unfired campaign contact events

1.7.0-mailroom-7.1.22
----------
 * Catalog message support

1.6.14-mailroom-7.1.22
----------
 * Update goflow version

1.6.13-mailroom-7.1.22
----------
 * Fix/order trigger resume

1.6.12-mailroom-7.1.22
----------
 * Changing function names for zeroshot variables

1.6.11-mailroom-7.1.22
----------
 * Add new environment variables for zeroshot api

1.6.10-mailroom-7.1.22
----------
 * Fix twilioflex panic on fetch media error

1.6.9-mailroom-7.1.22
----------
 * Wenichats open room receiving defined custom fields of body is properly configured or all fields if not

1.6.8-mailroom-7.1.22
----------
 * Fix zendesk ticketer client users endpoints

1.6.7-mailroom-7.1.22
----------
 * Relates a zendesk user with external id equal to contact uuid when opening zendesk ticket

1.6.6-mailroom-7.1.22
----------
 * Increase tests coverage in web module

1.6.5-mailroom-7.1.22
----------
 * Add support to receive order on input and tigger and any metadata object in trigger

1.6.4-mailroom-7.1.22
----------
 * Update goflow version for v0.1.1-goflow-0.144.3

1.6.3-mailroom-7.1.22
----------
 * Add close room webhook in parameters for RocketChat

1.6.2-mailroom-7.1.22
----------
 * Add email field to params 

1.6.1-mailroom-7.1.22
----------
 * Add ChatGPT configs for temperature and top_p

1.6.0-mailroom-7.1.22
----------
 * Add ChatGPT external service

1.5.8-mailroom-7.1.22
----------
 * Support for trigger.params in Msg events

1.5.7-mailroom-7.1.22
----------
 * Remove returned ticket in case of reopening for zendesk

1.5.6-mailroom-7.1.22
----------
 * Support for reopening zendesk ticket with body ID

1.5.5-mailroom-7.1.22
----------
 * Add support for sending webhook parameters in trigger.params to Zendesk

1.5.4-mailroom-7.1.22
----------
 * Fix goflow version command 

1.5.3-mailroom-7.1.22
----------
 * Send is_anon field when opening room in wenichats

1.5.2-mailroom-7.1.22
----------
 * Remove topups to fix tests

1.5.1-mailroom-7.1.22
----------
 * use contact id for name on open wenichats ticket if name is empty

1.5.0-mailroom-7.1.22
----------
 * Support for the new Omie external service

1.4.18-mailroom-7.1.22
----------
 * Fix wenichats send media #118
 * Ordering Twilioflex msg history and send each as a separated message #117

1.4.17-mailroom-7.1.22
----------
 * Send message attribute to Zendesk on attachment submissions #115
 * Tweak wenichats integration open room to pass FlowUUID and contact groups #113 & #114
 * Fix FetchFileWithMaxSize #113

1.4.16-mailroom-7.1.22
----------
 * Added wenichats webhook media body bytes limits #111

1.4.15-mailroom-7.1.22
----------
 * Send chatbot history on Twilio in just one message #109

1.4.14-mailroom-7.1.22
----------
 * Added config for flow start batch timeout #107

1.4.13-mailroom-7.1.22
----------
 * Add Domain to File URL for Zendesk #105

1.4.12-mailroom-7.1.22
----------
 * Fix file endpoint for Zendesk #103

1.4.11-mailroom-7.1.22
----------
 * Add support for sending contact language in messages for WAC and WA #101

1.4.10-mailroom-7.1.22
----------
 * Fix submitting tags and custom fields for Zendesk tickets #99

1.4.9-mailroom-7.1.22
----------
 * add contact urn field to wenichats room creation params #97

1.4.8-mailroom-7.1.22
----------
 * Fix twilio flex messages history #95

1.4.7-mailroom-7.1.22
----------
 * Fix twilio flex media creation content-type param #93

1.4.6-mailroom-7.1.22
----------
 * Fix tag registration, custom_fields and ticket closing in Zendesk #91

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
