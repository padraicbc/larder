# TODO

- [ ] Add a search-provider choice in Food chat: OpenAI web search or a search MCP built into the app.
  - Build the MCP search integration into Larder rather than requiring users to supply an existing remote MCP server.
  - Let users choose the search provider independently of the conversation model, with any required search-service settings made explicit.
  - Return source links and indicate which provider actually searched; report failures instead of silently substituting model knowledge.
  - Keep the chat → draft → review → explicit save workflow. Calculate product ratings per request; never persist percentages.
