# New User Moderation Features

## Overview

The Community Toolkit Plugin provides comprehensive moderation features to help protect your Mattermost community from spam, abuse, and disruptive behavior by new users. This document describes the new user restriction features that allow administrators to temporarily (or permanently) limit what newly registered users can do.

These features are designed to provide a "trust building" period where new users must participate in public channels before gaining full access to all communication features.

## Features

The plugin offers three independent moderation controls for new users:

1. **Direct Message (DM) Blocking** - Prevents new users from sending private/direct messages
2. **Link Blocking** - Prevents new users from posting URLs and links
3. **Image Blocking** - Prevents new users from posting images and media

Each feature can be:
- Enabled or disabled independently
- Configured with different time durations
- Set to block indefinitely

## Why Use New User Moderation?

### Common Use Cases

**Anti-Spam Protection**
- Prevents spammers from mass-DMing users
- Stops link spam in channels
- Blocks image-based spam and phishing

**Community Onboarding**
- Encourages new users to participate in public channels first
- Allows moderators to observe new user behavior
- Builds trust before granting full access

**High-Security Environments**
- Protects sensitive channels from social engineering
- Reduces phishing attack vectors
- Provides time to verify new user legitimacy

**Public Communities**
- Reduces harassment via DMs
- Prevents drive-by link spam
- Deters trolls and bad actors

## Configuration Guide

All settings are configured through the Mattermost System Console under:
**System Console → Plugins → Community Toolkit**

### Direct Message Blocking

**Setting: Block New User PMs**
- Type: Boolean (checkbox)
- Default: Disabled
- Description: When enabled, prevents new users from sending direct messages for the configured duration

**Setting: Block New User PM Time**
- Type: Text (duration string)
- Default: `24h`
- Description: How long to block DMs after account creation
- Special values:
  - `-1` = Block indefinitely (user can never send DMs)
  - Empty = Feature disabled

**Duration Format Examples:**
- `1h` = 1 hour
- `24h` = 24 hours (1 day)
- `168h` = 168 hours (7 days)
- `12h30m` = 12 hours and 30 minutes
- `-1` = Indefinite (permanent block)

### Link Blocking

**Setting: Block New User Links**
- Type: Boolean (checkbox)
- Default: Disabled
- Description: When enabled, prevents new users from posting links for the configured duration

**Setting: Block New User Links Time**
- Type: Text (duration string)
- Default: `24h`
- Description: How long to block link posts after account creation
- Format: Same as DM blocking (see above)

**What Counts as a Link:**
- URLs starting with `http://` or `https://`
- URLs starting with `www.`
- Posts with OpenGraph link previews
- Markdown-formatted links

**Note:** Plain domain names without protocol (e.g., "example.com") are NOT detected as links.

### Image Blocking

**Setting: Block New User Images**
- Type: Boolean (checkbox)
- Default: Disabled
- Description: When enabled, prevents new users from posting images for the configured duration

**Setting: Block New User Images Time**
- Type: Text (duration string)
- Default: `24h`
- Description: How long to block image posts after account creation
- Format: Same as DM blocking (see above)

**What Counts as an Image:**
- File attachments with image extensions (jpg, jpeg, png, gif, bmp, webp, svg, tiff, ico, heic, heif, avif)
- Embedded images from URLs
- Markdown-formatted images (`![alt](url)`)

## Configuration Examples

### Conservative Setup (Recommended for Most Communities)
Blocks new users from potentially disruptive actions for 24 hours:

```
Block New User PMs: ✓ Enabled
Block New User PM Time: 24h

Block New User Links: ✓ Enabled
Block New User Links Time: 24h

Block New User Images: ✓ Enabled
Block New User Images Time: 24h
```

**Effect:** New users can participate in public channels for 24 hours before gaining ability to send DMs, post links, or share images.

### Strict Setup (High-Security Communities)
Blocks new users for 7 days to allow thorough vetting:

```
Block New User PMs: ✓ Enabled
Block New User PM Time: 168h

Block New User Links: ✓ Enabled
Block New User Links Time: 168h

Block New User Images: ✓ Enabled
Block New User Images Time: 168h
```

**Effect:** New users must participate for a full week before gaining full access.

### Maximum Security (Permanent Restrictions)
Permanently blocks all new users from certain actions:

```
Block New User PMs: ✓ Enabled
Block New User PM Time: -1

Block New User Links: ✓ Enabled
Block New User Links Time: -1

Block New User Images: ✓ Enabled
Block New User Images Time: -1
```

**Effect:** New users can NEVER send DMs, post links, or share images. This effectively creates a "read-only" user base where only manually promoted users have full access.

**Warning:** This configuration is very restrictive. Consider using a trust level system or manual promotion process.

### Mixed Approach (Graduated Access)
Different durations for different features:

```
Block New User PMs: ✓ Enabled
Block New User PM Time: 168h (7 days for DMs)

Block New User Links: ✓ Enabled
Block New User Links Time: 24h (1 day for links)

Block New User Images: ✓ Enabled
Block New User Images Time: 48h (2 days for images)
```

**Effect:** New users gain features gradually - links after 1 day, images after 2 days, DMs after 7 days.

### Anti-Spam Only (Links and Images)
Blocks spam vectors but allows DMs:

```
Block New User PMs: ✗ Disabled

Block New User Links: ✓ Enabled
Block New User Links Time: 48h

Block New User Images: ✓ Enabled
Block New User Images Time: 48h
```

**Effect:** Prevents link/image spam but allows legitimate users to communicate via DM.

## User Experience

### What Users See

When a new user attempts a blocked action, they receive an ephemeral message (only visible to them) explaining why their action was blocked:

**For Direct Messages:**
> "Configuration settings limit new users from sending private messages."

**For Links:**
> "Configuration settings limit new users from posting links."

**For Images:**
> "Configuration settings limit new users from posting images."

The post is not created, and no notification is sent to other users.

### User Timeline

Here's what a new user experiences with the "Conservative Setup" (24h blocks):

**Hour 0 (Account Creation):**
- ✓ Can view all public channels
- ✓ Can post text messages in public channels
- ✗ Cannot send direct messages
- ✗ Cannot post links
- ✗ Cannot post images

**Hour 24 (After 24 hours):**
- ✓ All restrictions lifted automatically
- ✓ Can now send DMs, post links, and share images

**No manual intervention required** - restrictions lift automatically based on account age.

## Technical Details

### How Detection Works

**Link Detection:**
1. Checks for OpenGraph embeds in post metadata (most reliable)
2. Uses regex pattern to detect URLs: `https?://[^\s<>"]+|www\.[^\s<>"]+`
3. Matches in post message content

**Image Detection:**
1. Checks file attachments for image extensions (case-insensitive)
2. Checks for image embeds in post metadata
3. Detects Markdown image syntax: `![alt](url)`

**Supported Image Formats:**
- Common: jpg, jpeg, png, gif, bmp
- Modern: webp, avif, heic, heif
- Other: svg, tiff, tif, ico

**User Age Calculation:**
- Based on user's `CreateAt` timestamp from Mattermost database
- Calculated at time of post attempt
- Uses Go's `time.ParseDuration()` for parsing duration strings

### Duration Format Specification

The duration format follows Go's `time.ParseDuration()` standard:

**Valid Units:**
- `ns` = nanoseconds
- `us` or `µs` = microseconds
- `ms` = milliseconds
- `s` = seconds
- `m` = minutes
- `h` = hours

**Valid Formats:**
- Single unit: `24h`, `60m`, `3600s`
- Multiple units: `1h30m`, `2h45m30s`
- Decimal notation: `1.5h` (90 minutes)
- Special value: `-1` (indefinite)

**Invalid Formats:**
- Days: `7d` ❌ (use `168h` instead)
- Weeks: `1w` ❌ (use `168h` for 1 week)
- Spaces: `24 hours` ❌ (use `24h`)
- Text: `one day` ❌

**Configuration Validation:**
The plugin validates duration formats when configuration changes. Invalid formats will prevent the configuration from being saved and show an error message.

### Performance Considerations

**Caching:**
- User objects are cached in a 50-entry LRU cache
- Reduces database queries for frequently active users
- Cache hit rate typically >90% in active channels

**Regex Patterns:**
- Compiled once when configuration changes
- Reused for all post checks (no per-post compilation cost)
- Minimal performance impact on posting

**Recommended Limits:**
- Plugin handles thousands of posts per second
- No significant performance impact observed
- Safe for communities of any size

## Troubleshooting

### Common Issues

**Issue: New users are not being blocked**

**Solutions:**
1. Verify the feature is enabled (checkbox is checked)
2. Check that duration is set (not empty)
3. Ensure duration format is valid (e.g., `24h`, not `24 hours`)
4. Check plugin logs for errors
5. Verify plugin is active and enabled

**Issue: Old users are being blocked**

**Solutions:**
1. Check if duration is set to `-1` (indefinite blocking affects ALL users)
2. Verify user creation date in Mattermost database
3. Check for clock sync issues on server
4. Review plugin cache (may need plugin restart)

**Issue: Links are not being detected**

**Reasons:**
1. Plain domain names without protocol are not detected (by design)
2. Obfuscated URLs may bypass detection
3. Links in code blocks are still detected (feature limitation)

**Solutions:**
- Educate users that `www.example.com` IS detected
- Consider using word filtering for domain names
- Report bypass techniques to plugin maintainers

**Issue: Configuration won't save**

**Causes:**
1. Invalid duration format
2. Plugin configuration error
3. Insufficient permissions

**Solutions:**
1. Verify duration format (see "Duration Format Specification")
2. Check Mattermost logs for plugin errors
3. Ensure you have System Admin permissions
4. Try disabling and re-enabling the plugin

### Checking if Features Are Working

**Test Link Blocking:**
1. Create a test user account
2. Immediately try posting a link: `https://example.com`
3. Should receive blocking message

**Test Image Blocking:**
1. Create a test user account
2. Immediately try uploading an image file
3. Should receive blocking message

**Test DM Blocking:**
1. Create a test user account
2. Immediately try sending a DM to another user
3. Should receive blocking message

**Verify Duration:**
1. Note the time you create the test account
2. Wait for the configured duration to pass
3. Try the action again - should now be allowed

### Debug Commands

**Check Plugin Status:**
```bash
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  $SITEURL/api/v4/plugins | jq '.active'
```

**View Plugin Logs:**
```bash
# If using pluginctl
make logs

# Or via Mattermost logs
tail -f /opt/mattermost/logs/mattermost.log | grep community-toolkit
```

**Check User Creation Time:**
```sql
-- In Mattermost database
SELECT Id, Username, CreateAt, FROM_UNIXTIME(CreateAt/1000) as CreatedDate
FROM Users
WHERE Username = 'test_user';
```

## Best Practices

### Recommended Configurations

**For Public Communities:**
- Start with 24h blocks on all features
- Monitor for false positives
- Adjust durations based on community culture

**For Private/Corporate Instances:**
- Consider shorter durations (1-4 hours)
- Or disable DM blocking entirely
- Focus on link/image spam prevention

**For High-Risk Communities:**
- Use 7-day (168h) blocks
- Enable email domain filtering
- Combine with username validation

### Communication Strategy

**Set Expectations:**
- Document restrictions in welcome messages
- Update registration email/onboarding materials
- Post rules in welcome channel

**Example Welcome Message:**
> "Welcome to our community! To prevent spam, new accounts have temporary restrictions:
> - You can post in public channels immediately
> - After 24 hours, you can send direct messages and post links/images
>
> Thank you for your patience as we keep our community safe!"

**Monitor User Feedback:**
- Watch for complaints from legitimate users
- Adjust durations if onboarding is too restrictive
- Consider graduated access approach

### Security Considerations

**Defense in Depth:**
- Don't rely solely on new user restrictions
- Combine with email domain filtering
- Use username validation
- Enable built-in profanity filter

**Legitimate User Impact:**
- Shorter durations (1-24h) minimize frustration
- Avoid indefinite blocks unless necessary
- Consider manual "verified user" process for trusted accounts

**Bypass Considerations:**
- Attackers may create accounts in advance (age them)
- Monitor for coordinated attacks from multiple aged accounts
- Supplement with rate limiting and behavior analysis

## Future Enhancements

The following features are planned for future releases:

- **Trust Levels:** Manual promotion system for verified users
- **LDAP Integration:** Bypass restrictions for LDAP/SSO users
- **Graduated Permissions:** Fine-grained control over feature access
- **Moderation Dashboard:** UI for viewing blocked attempts
- **Analytics:** Reports on blocked content and users
- **Custom Messages:** Configurable user-facing messages per restriction

## Support and Feedback

For issues, feature requests, or questions:
- GitHub Issues: [mattermost-plugin-community-toolkit/issues](https://github.com/rocky-linux/mattermost-plugin-community-toolkit/issues)
- Documentation: See main README.md
- Community: Mattermost Community Server

## Appendix: Configuration Reference

### Complete Settings Matrix

| Setting | Type | Default | Valid Values | Description |
|---------|------|---------|--------------|-------------|
| BlockNewUserPM | Boolean | false | true/false | Enable DM blocking |
| BlockNewUserPMTime | String | "24h" | duration or "-1" | DM block duration |
| BlockNewUserLinks | Boolean | false | true/false | Enable link blocking |
| BlockNewUserLinksTime | String | "24h" | duration or "-1" | Link block duration |
| BlockNewUserImages | Boolean | false | true/false | Enable image blocking |
| BlockNewUserImagesTime | String | "24h" | duration or "-1" | Image block duration |

### Duration Conversion Chart

| Human Readable | Duration String |
|----------------|-----------------|
| 1 hour | `1h` or `60m` |
| 6 hours | `6h` |
| 12 hours | `12h` |
| 24 hours (1 day) | `24h` |
| 48 hours (2 days) | `48h` |
| 72 hours (3 days) | `72h` |
| 168 hours (1 week) | `168h` |
| 336 hours (2 weeks) | `336h` |
| 720 hours (30 days) | `720h` |
| Indefinite | `-1` |

---

**Document Version:** 1.0
**Last Updated:** 2025-10-31
**Compatible with:** Community Toolkit Plugin v2.0.6+
