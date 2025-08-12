# coralogix-api-client
Creates an outgoing webhook and an alert connected to that webhook
prints ids of both to console

## part 3 discussion
### 1. api quality evaluation:
- the api is very comprehensive and flexible
- it's openapi

- might be too comprehensive for single endpoint - hard to know which one to use
- from what I could see, there is no way to just attach alert to outgoing webhook - must reconfigure the alert with same configuration with added webhook if we want to do that
- some unclear options (UNKNOWN option in unexpected places), was hard to tell if I should use "externalID" vs "ID" of webhook


### 2. suggested improvements:
- can split the endpoint by source type (logs alerts, traces alerts, etc) which could avoid some confusion
- endpoint to attach alert to a webhook (or any notification entity for that matter)
- some documentation or naming changes could help clarify potential mixups. for example "external id" == "integration id"