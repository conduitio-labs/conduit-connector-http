# Conduit Connector for Discord
[Conduit](https://conduit.io) source connector for discord.

Create a Discord App and Bot, then  add the bot to your server, This is beyond the scope of this README.
You can find more details in the [Discord Developer Portal](https://discord.com/developers/docs/intro).


## How to build?
Run `make build` to build the connector.

## Testing
Run `make test` to run all the unit tests. 

## Source
A Discord source connector pulls messages from the discord channel every `pollingPeriod`, it returns one message each time
the `Read()` method is called.

It starts by returning all the message history from the channel one by one, then listens for new messages after that.
The record position is set to the latest `message-id`, so that it would only read messages from after the position.

### Configuration

| name            | description                                                                                        | required | default value |
|-----------------|----------------------------------------------------------------------------------------------------|----------|---------------|
| `channel-id`    | get the `channel-id` by enabling developer mode and right click on the channel name to get the id. | true     |               |
| `token`         | the Bot Token.                                                                                     | true     |               |
| `pollingPeriod` | how often the connector will read messages from the channel, formatted as a time.Duration string.  | false    | 5m            |
