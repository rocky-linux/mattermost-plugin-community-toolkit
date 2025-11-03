# Mattermost Community Toolkit Plugin (Beta)

(based on the [mattermost-plugin-profanity-filter](https://github.com/mattermost/mattermost-plugin-profanity-filter) by Mattermost)

This plugin allows you to manage multiple settings relating to preventing spam and misuse of a Mattermost server. This plugin uses the profanity-filter plugin developed by Mattermost as a base to provide a comprehensive toolkit to manage your Mattermost community at scale.

The plugin has the following features:

* Censor/filter posts on the server (including during editing) to either reject or censor unwanted words (e.g., profanity)
    * Words can be replaced with a series of characters (e.g., "\*"), or rejected outright with a message to the user
* Automatically deactivate users (cancel registration) if their username matches list of unwanted names
* Automatically deactivate users (cancel registration) if their email matches list of unwanted domains/addresses
* Prevent new users from sending direct messages to other users for some time period

In the future, this plugin will:

* Send notifications to a centralized channel of moderation actions taken
* Allow moderators to restore accounts, perform inquiries on users, see the history of the account and its changes
* Grant "trust" levels to users based on the account status and optional moderator input
    * e.g., allow accounts in a certain LDAP group to bypass checks
* Be a hub for all community operations activities--moderation and otherwise

**Supported Mattermost Server Versions: 9.3+**

## Manual Installation

1. Go to the [releases page of this Github repository](https://github.com/rocky-linux/mattermost-plugin-community-toolkit/releases) and download the latest release for your Mattermost server.
2. Upload this file in the Mattermost System Console under **System Console > Plugins > Management** to install the plugin. To learn more about how to upload a plugin, [see the documentation](https://docs.mattermost.com/administration/plugins.html#plugin-uploads).
3. Activate the plugin at **System Console > Plugins > Management**.

### Usage

You can edit the bad words list in **System Console > Plugins > Profanity Filter > Bad words list**.
In this list, you can use Regular Expressions to match bad words. For example, `bad[[:space:]]?word` will match both `badword` and `bad word`.

Choose to either censor the bad words with a character or reject the post with a custom warning message:

![Post rejected by the plugin](./images/post-rejected.gif)

![Post censored by the plugin](./images/post-censored.gif)

In addition to the Bad Word List, a Bad Domain and Bad Username list is available to configure. The Bad Domain list is prepopulated with a [list](https://github.com/unkn0w/disposable-email-domain-list) of known disposable email addresses. Both the domain and username lists support regular expressions.

## Contributing

Want to help improve the Mattermost Community Toolkit Plugin? Please see our [Contributing Guide](CONTRIBUTING.md) for detailed information on:

* Development environment setup
* Build system and make commands
* Debugging and troubleshooting
