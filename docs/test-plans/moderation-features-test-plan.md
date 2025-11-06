# Release Validation Test Plan

## Purpose

This document provides manual release validation tests for the Mattermost Community Toolkit plugin. These tests are designed to verify that all moderation features work correctly and are suitable for someone new to Mattermost.

**Note:** This test plan covers high-level validation tests only. Comprehensive depth testing is covered by automated test suites.

## Prerequisites

- Local development environment set up and running (see `docs/PODMAN_DEVELOPMENT.md`)
- Mattermost accessible at `http://localhost:8065`
- Plugin installed and enabled
- Admin access to System Console
- Basic familiarity with command line
- Podman Compose installed (for database access)

---

## Feature Overview

The Community Toolkit plugin provides six main moderation features:

### 1. Bad Word Filtering

Filters posts containing profanity or offensive words. Can operate in two modes:

- **Censor Mode** (default): Replaces bad words with censor characters (e.g., `****`)
- **Reject Mode**: Completely blocks the post and shows a warning message

**Key Configuration Options:**

- Bad Words List: Comma-separated list of words/patterns (supports regex)
- Reject Posts: Toggle between censor and reject modes
- Censor Character: Character(s) to use for replacement (default: `*`)
- Warning Message: Custom message shown when posts are rejected
- Exclude Bots: Option to skip filtering for bot messages

### 2. New User Direct Message Blocking

Prevents newly registered users from sending direct/private messages for a configured time period.

**Key Configuration Options:**

- Block New User PMs: Enable/disable feature
- Block New User PM Time: Duration (e.g., `24h`, `7d`) or `-1` for indefinite

### 3. New User Link Blocking

Prevents newly registered users from posting URLs and links.

**Key Configuration Options:**

- Block New User Links: Enable/disable feature
- Block New User Links Time: Duration or `-1` for indefinite

### 4. New User Image Blocking

Prevents newly registered users from posting images and media files.

**Key Configuration Options:**

- Block New User Images: Enable/disable feature
- Block New User Images Time: Duration or `-1` for indefinite

### 5. Username Validation

Automatically blocks and cleans up users with inappropriate usernames or nicknames during registration.

**Key Configuration Options:**

- Bad Usernames: Comma-separated list of username patterns (supports regex)

### 6. Email Domain Validation

Automatically blocks and cleans up users registering with disposable or blocked email domains.

**Key Configuration Options:**

- Use Built-in Bad-Domains list: Enable built-in disposable domain list (51,501 domains)
- Bad Domains List: Additional custom domain patterns (supports regex)

---

## Test Setup

### Step 1: Start Development Environment

```bash
# From the plugin repository root directory
make dev-up
```

Wait for Mattermost to be ready (usually 30-60 seconds). You can verify it's running by opening `http://localhost:8065` in your browser.

### Step 2: Access Mattermost

1. Open `http://localhost:8065` in your web browser
2. If this is the first time, complete the initial setup wizard to create your admin account
3. Log in with your admin credentials

### Step 3: Install and Enable Plugin

1. **Build the plugin:**

   ```bash
   make dist
   ```

2. **Deploy the plugin:**

   ```bash
   make dev-deploy
   ```

3. **Enable the plugin:**
   - Navigate to **System Console** (click hamburger menu ☰ → System Console)
   - Go to **Plugins** → **Community Toolkit**
   - Click **Enable** button
   - Wait for "Plugin enabled successfully" message

### Step 4: Configure Plugin

1. In System Console, go to **Plugins** → **Community Toolkit**
2. Configure settings as needed for each test (specific configurations will be provided in each test case)
3. Click **Save** after making changes

---

## Creating Test Users

### Method 1: Via the `mmctl` tool (Recommended)

1. In the terminal where you started the local development environment
2. Establish a credential for using the `mmctl` command:

   ```bash
   podman-compose exec mattermost bin/mmctl auth login http://localhost:8065
   ```

   This will ask you for three pieces of information: 
   - Connection Name: Use anything you want here, example: `testconn`
   - User Name: Use the admin user, default is: `admin`
   - User Password: Use the password the admin user was setup with, default is: `admin123`

3. Run (substitute the values for the user you want to create):

   ```bash
   podman-compose exec mattermost bin/mmctl user create \
   --email test01@example.com \
   --username test01 \
   --password UserPassword123
   ```

**Note:** Users created this way will have a "creation date" of the current time. To test time-based restrictions, you'll need to modify the creation date in the database (see below).

### Method 2: Via User Registration (For Email/Username Validation Tests)

1. Navigate to **System Console** → **Authentication** → **Sign Up**
2. Ensure "Enable account creation" is enabled
3. Use the registration page at `http://localhost:8065/signup_user_complete`
4. Register with the test email/username you want to validate

---

## Modifying User Creation Date (Simulating User Age)

To test time-based restrictions (DM blocking, link blocking, image blocking), you need to simulate users of different ages. This requires modifying the `CreateAt` field in the PostgreSQL database.

### Step 1: Access PostgreSQL Container

```bash
# From the plugin repository root directory
podman-compose exec postgres psql -U mmuser -d mattermost
```

You should see a `mattermost=#` prompt.

### Step 2: Find Your Test User

```sql
-- List users to find the one you want to modify
SELECT id, username, email, to_timestamp(createat/1000) as created_at
FROM users
WHERE deleteat = 0
ORDER BY createat DESC;
```

Look for your test user in the output. Note the `id` (a long string) and `createat` (a large number in milliseconds).

### Step 3: Calculate New CreateAt Value

The `CreateAt` field is stored as milliseconds since Unix epoch (January 1, 1970).

**Examples:**

- To set user to be created **2 hours ago** (for testing 1-hour restriction):

  ```sql
  -- Current timestamp in milliseconds minus 2 hours
  SELECT (EXTRACT(EPOCH FROM NOW() - INTERVAL '2 hours') * 1000)::bigint;
  ```

- To set user to be created **25 hours ago** (for testing 24-hour restriction):

  ```sql
  -- Current timestamp in milliseconds minus 25 hours
  SELECT (EXTRACT(EPOCH FROM NOW() - INTERVAL '25 hours') * 1000)::bigint;
  ```

- To set user to be created **just now** (for testing new user restrictions):
  ```sql
  -- Current timestamp in milliseconds
  SELECT (EXTRACT(EPOCH FROM NOW()) * 1000)::bigint;
  ```

### Step 4: Update User Creation Date

Replace `USER_ID_HERE` with your user's ID from Step 2, and `NEW_CREATEAT_VALUE` with the calculated value:

```sql
UPDATE users
SET createat = NEW_CREATEAT_VALUE
WHERE id = 'USER_ID_HERE';
```

**Example:** Set a user to be 25 hours old:

```sql
UPDATE users
SET createat = (EXTRACT(EPOCH FROM NOW() - INTERVAL '25 hours') * 1000)::bigint
WHERE username = 'testuser1';
```

### Step 5: Verify the Change

```sql
SELECT username, to_timestamp(createat/1000) as created_at,
       NOW() - to_timestamp(createat/1000) as age
FROM users
WHERE username = 'testuser1';
```

### Step 6: Exit PostgreSQL

```sql
\q
```

**Important Notes:**

- User age calculations are done at post time, so changes take effect immediately
- The plugin uses `time.Since(user.CreateAt)` to determine user age
- Always verify the user's age after modification
- If testing fails, verify the timestamp was updated correctly

---

## Test Cases

### Test Suite 1: Bad Word Filtering

#### Test 1.1: Word Censoring (Default Mode)

**Objective:** Verify that bad words are replaced with censor characters when Reject Posts is disabled.

**Prerequisites:**

- Plugin enabled
- System Console → Plugins → Community Toolkit:
  - **Reject Posts**: Unchecked (disabled)
  - **Censor Character**: `*` (default)
  - **Bad Words List**: `testword,badword` (add simple test words)

**Test Steps:**

1. Log in as a regular user (not admin)
2. Navigate to any channel
3. Post a message containing a test bad word, e.g., `This is a testword message`
4. Observe the post after it appears

**Expected Results:**

- The bad word `testword` should be replaced with `********` (8 asterisks, one per character)
- The rest of the message should remain unchanged
- The post should appear in the channel

**Pass/Fail Criteria:**

- ✅ **PASS**: Bad word is censored with asterisks
- ❌ **FAIL**: Bad word appears in full, or post is rejected

---

#### Test 1.2: Word Rejection Mode

**Objective:** Verify that posts containing bad words are completely rejected when Reject Posts is enabled.

**Prerequisites:**

- Plugin enabled
- System Console → Plugins → Community Toolkit:
  - **Reject Posts**: Checked (enabled)
  - **Warning Message**: Default or custom message
  - **Bad Words List**: `testword,badword`

**Test Steps:**

1. Log in as a regular user
2. Navigate to any channel
3. Type a message containing a bad word: `This contains testword`
4. Click **Send** (or press Enter)
5. Observe what happens

**Expected Results:**

- The message should NOT appear in the channel
- An ephemeral (temporary) warning message should appear, saying something like: "Your post has been rejected by the Profanity Filter, because the following word is not allowed: `testword`."
- The ephemeral message should disappear after a few seconds

**Pass/Fail Criteria:**

- ✅ **PASS**: Post is blocked and warning message appears
- ❌ **FAIL**: Post appears in channel, or no warning message shown

---

#### Test 1.3: Bot Exclusion

**Objective:** Verify that bot messages are not filtered when Exclude Bots is enabled.

**Prerequisites:**

- Plugin enabled
- System Console → Plugins → Community Toolkit:
  - **Exclude Bots**: Checked (enabled)
  - **Bad Words List**: `testword,badword`
- A bot account created (or use a test bot)

**Test Steps:**

1. As an admin, create a bot account:
   - First, enable bot accounts: System Console → Integrations → Bot Accounts → Enable Bot Account Creation
   - Then create the bot: System Console → Integrations → Bot Accounts → Add Bot Account
   - Give it a username and display name
   - After creation, create a Personal Access Token for the bot
   - Save and note the bot's access token
2. Add the bot to a known channel (`Town Square` is a good choice)
3. Get your team ID first, then find the channel ID for `Town Square`:

   ```bash
   # First, get your team ID (replace YOUR_TEAM_NAME with your actual team name)
   curl -X GET "http://localhost:8065/api/v4/teams/name/YOUR_TEAM_NAME" \
   -H "Authorization: Bearer BOT_ACCESS_TOKEN_HERE" | jq -r '.id'

   # Then, get the channel ID using the team ID
   curl -X GET "http://localhost:8065/api/v4/teams/TEAM_ID_HERE/channels/name/town-square" \
   -H "Authorization: Bearer BOT_ACCESS_TOKEN_HERE" | jq -r '.id'
   ```

   **Note:** Your team name is created during initial Mattermost setup. Common default team names include the site name or organization name you provided during setup.

4. Using the channel ID you can post messages into the channel as the bot:

   ```bash
   curl -X POST "http://localhost:8065/api/v4/posts" \
   -H "Authorization: Bearer BOT_ACCESS_TOKEN_HERE" \
   -H "Content-Type: application/json" \
   -d '{
      "channel_id": "CHANNEL_ID_GOES_HERE",
      "message": "Hello from the bot! testword"
   }'
   ```

   **Note:** Replace `testword` with a bad word from your configuration.

**Expected Results:**

- Bot message with bad word should appear uncensored
- Bot messages bypass the filter

**Pass/Fail Criteria:**

- ✅ **PASS**: Bot message appears with bad word uncensored
- ❌ **FAIL**: Bot message is censored or rejected

---

#### Test 1.4: Case-Insensitive Matching

**Objective:** Verify that bad word detection works regardless of capitalization.

**Prerequisites:**

- Plugin enabled
- **Bad Words List**: `testword`
- **Reject Posts**: Disabled (to see censoring)

**Test Steps:**

1. Log in as a regular user
2. Post messages with variations:
   - `TESTWORD`
   - `TestWord`
   - `testword`
   - `TeStWoRd`

**Expected Results:**

- All variations should be detected and censored/rejected
- Matching should be case-insensitive

**Pass/Fail Criteria:**

- ✅ **PASS**: All capitalization variations are filtered
- ❌ **FAIL**: Some variations bypass the filter

---

#### Test 1.5: Custom Censor Character

**Objective:** Verify that custom censor characters work correctly.

**Prerequisites:**

- Plugin enabled
- **Censor Character**: `X` (or another character)
- **Reject Posts**: Disabled
- **Bad Words List**: `testword`

**Test Steps:**

1. Log in as a regular user
2. Post a message: `This is a testword message`

**Expected Results:**

- Bad word should be replaced with `XXXXXXXX` (8 X's, one per character)
- Custom character is used instead of default `*`

**Pass/Fail Criteria:**

- ✅ **PASS**: Bad word censored with custom character
- ❌ **FAIL**: Default asterisk used, or wrong number of characters

---

### Test Suite 2: New User Direct Message Blocking

#### Test 2.1: New User Cannot Send DM

**Objective:** Verify that a newly created user cannot send direct messages.

**Prerequisites:**

- Plugin enabled
- System Console → Plugins → Community Toolkit:
  - **Block New User PMs**: Checked (enabled)
  - **Block New User PM Time**: `24h`

**Test Steps:**

1. Create a new test user (created just now, no age modification needed)
2. Log in as the new test user
3. Try to send a direct message to another user:
   - Click the "+" next to Direct Messages in the left sidebar
   - Search for another user
   - Start typing a message
   - Click Send
4. Observe what happens

**Expected Results:**

- Message should NOT be sent
- An ephemeral warning message should appear: "Configuration settings limit new users from sending private messages."
- The DM should not appear in either user's message list

**Pass/Fail Criteria:**

- ✅ **PASS**: DM is blocked and warning message appears
- ❌ **FAIL**: DM is sent successfully, or no warning message

---

#### Test 2.2: Older User Can Send DM

**Objective:** Verify that users older than the configured time can send DMs.

**Prerequisites:**

- Plugin enabled
- **Block New User PMs**: Enabled
- **Block New User PM Time**: `24h`
- A test user that is 25+ hours old (modify creation date as described earlier)

**Test Steps:**

1. Log in as the older test user (created 25+ hours ago)
2. Send a direct message to another user
3. Observe if the message is sent

**Expected Results:**

- Message should be sent successfully
- No warning message should appear
- DM should appear in both users' message lists

**Pass/Fail Criteria:**

- ✅ **PASS**: DM is sent successfully without warnings
- ❌ **FAIL**: DM is blocked, or warning message appears

---

#### Test 2.3: Indefinite Blocking (-1)

**Objective:** Verify that indefinite blocking prevents DMs permanently.

**Prerequisites:**

- Plugin enabled
- **Block New User PMs**: Enabled
- **Block New User PM Time**: `-1` (indefinite)

**Test Steps:**

1. Create a test user (any age, doesn't matter for indefinite blocking)
2. Log in as the test user
3. Try to send a DM (even if user is old)
4. Observe what happens

**Expected Results:**

- DM should be blocked regardless of user age
- Warning message should appear

**Pass/Fail Criteria:**

- ✅ **PASS**: DM is blocked even for old users
- ❌ **FAIL**: Old users can send DMs

---

### Test Suite 3: New User Link Blocking

#### Test 3.1: New User Cannot Post HTTP URL

**Objective:** Verify that new users cannot post HTTP links.

**Prerequisites:**

- Plugin enabled
- **Block New User Links**: Enabled
- **Block New User Links Time**: `24h`
- A test user created just now (new user)

**Test Steps:**

1. Log in as the new test user
2. Navigate to any channel
3. Post a message: `Check out http://example.com`
4. Click Send

**Expected Results:**

- Message should NOT appear
- Ephemeral warning: "Configuration settings limit new users from posting links."
- Link is blocked

**Pass/Fail Criteria:**

- ✅ **PASS**: Link is blocked and warning appears
- ❌ **FAIL**: Link is posted successfully

---

#### Test 3.2: New User Cannot Post HTTPS URL

**Objective:** Verify HTTPS links are also blocked.

**Prerequisites:**

- Same as Test 3.1

**Test Steps:**

1. Log in as new test user
2. Post: `Visit https://example.com`

**Expected Results:**

- Message blocked
- Warning message appears

**Pass/Fail Criteria:**

- ✅ **PASS**: HTTPS link is blocked
- ❌ **FAIL**: HTTPS link bypasses filter

---

#### Test 3.3: New User Cannot Post www URL

**Objective:** Verify www-prefixed URLs are detected.

**Prerequisites:**

- Same as Test 3.1

**Test Steps:**

1. Log in as new test user
2. Post: `See www.example.com`

**Expected Results:**

- Message blocked
- Warning message appears

**Pass/Fail Criteria:**

- ✅ **PASS**: www URL is blocked
- ❌ **FAIL**: www URL bypasses filter

---

#### Test 3.4: Older User Can Post Links

**Objective:** Verify users older than the restriction can post links.

**Prerequisites:**

- **Block New User Links**: Enabled
- **Block New User Links Time**: `24h`
- Test user created 25+ hours ago (modify creation date)

**Test Steps:**

1. Log in as older test user
2. Post: `Check out https://example.com`

**Expected Results:**

- Link should post successfully
- No warning message

**Pass/Fail Criteria:**

- ✅ **PASS**: Older user can post links
- ❌ **FAIL**: Older user is blocked

---

### Test Suite 4: New User Image Blocking

#### Test 4.1: New User Cannot Attach Image File

**Objective:** Verify that new users cannot upload image files.

**Prerequisites:**

- Plugin enabled
- **Block New User Images**: Enabled
- **Block New User Images Time**: `24h`
- New test user

**Test Steps:**

1. Log in as new test user
2. Navigate to any channel
3. Click the attachment icon (paperclip) or drag and drop an image file
4. Select an image file (JPG, PNG, etc.)
5. Try to post the message with the image

**Expected Results:**

- Message should NOT be posted
- Warning: "Configuration settings limit new users from posting images."
- Image attachment is blocked

**Pass/Fail Criteria:**

- ✅ **PASS**: Image upload is blocked
- ❌ **FAIL**: Image is posted successfully

---

#### Test 4.2: New User Cannot Post Markdown Image

**Objective:** Verify markdown image syntax is detected.

**Prerequisites:**

- Same as Test 4.1

**Test Steps:**

1. Log in as new test user
2. Post: `![alt text](https://example.com/image.jpg)`

**Expected Results:**

- Message blocked
- Warning message appears

**Pass/Fail Criteria:**

- ✅ **PASS**: Markdown image is blocked
- ❌ **FAIL**: Markdown image bypasses filter

---

#### Test 4.3: Older User Can Post Images

**Objective:** Verify older users can post images.

**Prerequisites:**

- **Block New User Images**: Enabled
- **Block New User Images Time**: `24h`
- Test user 25+ hours old

**Test Steps:**

1. Log in as older test user
2. Upload an image file or post markdown image

**Expected Results:**

- Image should post successfully
- No warning message

**Pass/Fail Criteria:**

- ✅ **PASS**: Older user can post images
- ❌ **FAIL**: Older user is blocked

---

### Test Suite 5: Username Validation

#### Test 5.1: Bad Username Triggers Account Cleanup

**Objective:** Verify that users with bad usernames are automatically cleaned up.

**Prerequisites:**

- Plugin enabled
- **Bad Usernames**: `baduser,testbad` (add test patterns)
- User registration enabled (for this test)

**Test Steps:**

1. Ensure user registration is enabled (System Console → Authentication → Sign Up)
2. Navigate to registration page: `http://localhost:8065/signup_user_complete`
3. Register a new user with:
   - **Username**: `baduser` (or another pattern from your list)
   - **Email**: `baduser@example.com`
   - **Password**: (any secure password)
4. Complete registration
5. Try to log in with the new account

**Expected Results:**

- Registration may complete, but account should be immediately cleaned up
- User should NOT be able to log in (account is deactivated)
- In System Console → Users, the user should show as deactivated/deleted
- Username should be changed to `sanitized-{userid}` format

**Pass/Fail Criteria:**

- ✅ **PASS**: Account is deactivated and username sanitized
- ❌ **FAIL**: Account remains active with original username

---

#### Test 5.2: Good Username Allows Registration

**Objective:** Verify that valid usernames allow normal registration.

**Prerequisites:**

- Same as Test 5.1, but use a username NOT in the bad list

**Test Steps:**

1. Register a new user with username: `validuser`
2. Complete registration
3. Log in with the new account

**Expected Results:**

- Registration should complete successfully
- User should be able to log in
- Username should remain unchanged
- Account should be active

**Pass/Fail Criteria:**

- ✅ **PASS**: Account is active with original username
- ❌ **FAIL**: Valid username triggers cleanup

---

#### Test 5.3: Regex Pattern Matching

**Objective:** Verify that regex patterns in username list work correctly.

**Prerequisites:**

- **Bad Usernames**: `test.*bad` (regex pattern)
- User registration enabled

**Test Steps:**

1. Register users with:
   - Username: `test123bad` (should match pattern)
   - Username: `testbad` (should match)
   - Username: `testgood` (should NOT match)

**Expected Results:**

- `test123bad` and `testbad` should trigger cleanup
- `testgood` should be allowed

**Pass/Fail Criteria:**

- ✅ **PASS**: Regex patterns match correctly
- ❌ **FAIL**: Pattern matching fails

---

### Test Suite 6: Email Domain Validation

#### Test 6.1: Disposable Domain Blocks Registration

**Objective:** Verify that disposable email domains are blocked.

**Prerequisites:**

- Plugin enabled
- **Use Built-in Bad-Domains list**: Enabled
- User registration enabled
- Find a domain from the built-in list (common disposable domains include `10minutemail.com`, `tempmail.com`, etc.)

**Test Steps:**

1. Try to register with email from a disposable domain:
   - Email: `test@10minutemail.com` (or another disposable domain)
   - Username: `validusername`
   - Complete registration

**Expected Results:**

- Registration may complete, but account should be cleaned up immediately
- Account should be deactivated
- User cannot log in

**Pass/Fail Criteria:**

- ✅ **PASS**: Disposable domain triggers cleanup
- ❌ **FAIL**: Account remains active with disposable email

---

#### Test 6.2: Custom Domain Pattern Blocks Registration

**Objective:** Verify custom domain patterns work.

**Prerequisites:**

- **Bad Domains List**: `.*spam.*` (regex pattern)
- User registration enabled

**Test Steps:**

1. Register with email: `user@spamdomain.com`
2. Register with email: `user@example.com` (should NOT match)

**Expected Results:**

- `spamdomain.com` should trigger cleanup
- `example.com` should be allowed

**Pass/Fail Criteria:**

- ✅ **PASS**: Custom patterns match correctly
- ❌ **FAIL**: Pattern matching fails

---

#### Test 6.3: Good Domain Allows Registration

**Objective:** Verify legitimate domains allow registration.

**Prerequisites:**

- **Bad Domains List**: Only test patterns (not blocking legitimate domains)
- User registration enabled

**Test Steps:**

1. Register with email: `user@example.com`
2. Complete registration and log in

**Expected Results:**

- Registration succeeds
- Account remains active
- User can log in normally

**Pass/Fail Criteria:**

- ✅ **PASS**: Legitimate domain allows registration
- ❌ **FAIL**: Legitimate domain triggers cleanup

---

## Troubleshooting

### Plugin Not Working

**Symptoms:** No filtering happening, no restrictions applied.

**Solutions:**

1. Verify plugin is enabled:
   - System Console → Plugins → Community Toolkit → Check "Enabled" status
2. Check plugin logs:
   ```bash
   make dev-plugin-logs
   ```
3. Verify configuration is saved:
   - Make changes in System Console → Save → Verify changes persist
4. Restart plugin:
   ```bash
   make dev-reset
   ```

### User Age Calculations Not Working

**Symptoms:** Time-based restrictions don't work correctly.

**Solutions:**

1. Verify user creation date in database:
   ```sql
   SELECT username, to_timestamp(createat/1000) as created_at
   FROM users WHERE username = 'testuser';
   ```
2. Check if timestamp was updated correctly
3. Verify the plugin configuration for time duration (e.g., `24h`, not `24 hours`)
4. Wait a few seconds after database update (plugin caches user data)

### Can't Access Database

**Symptoms:** `podman-compose exec postgres` command fails.

**Solutions:**

1. Verify Podman containers are running:
   ```bash
   make dev-status
   ```
2. Try accessing container directly:
   ```bash
   podman exec -it mattermost-plugin-community-toolkit-postgres-1 psql -U mmuser -d mattermost
   ```
   (Container name may vary)

### Configuration Not Saving

**Symptoms:** Settings don't persist after saving.

**Solutions:**

1. Check for validation errors (red text in System Console)
2. Verify all required fields are filled
3. Check browser console for JavaScript errors
4. Try refreshing the page and checking again

### Test User Cannot Log In

**Symptoms:** After account cleanup, user cannot log in.

**Expected Behavior:** This is correct - cleaned up accounts are soft-deleted and cannot log in. To restore:

1. System Console → Users → Find deactivated user
2. Click on user → Restore

---

## Test Execution Checklist

### Pre-Test Setup

- [ ] Development environment started (`make dev-up`)
- [ ] Mattermost accessible at `http://localhost:8065`
- [ ] Admin account created and logged in
- [ ] Plugin installed and enabled
- [ ] Plugin configuration accessed (System Console → Plugins → Community Toolkit)

### Feature Testing Checklist

#### Bad Word Filtering

- [ ] Test 1.1: Word censoring works
- [ ] Test 1.2: Word rejection works
- [ ] Test 1.3: Bot exclusion works
- [ ] Test 1.4: Case-insensitive matching works
- [ ] Test 1.5: Custom censor character works

#### New User DM Blocking

- [ ] Test 2.1: New user cannot send DM
- [ ] Test 2.2: Older user can send DM
- [ ] Test 2.3: Indefinite blocking works

#### New User Link Blocking

- [ ] Test 3.1: HTTP links blocked for new users
- [ ] Test 3.2: HTTPS links blocked for new users
- [ ] Test 3.3: www URLs blocked for new users
- [ ] Test 3.4: Older users can post links

#### New User Image Blocking

- [ ] Test 4.1: Image attachments blocked for new users
- [ ] Test 4.2: Markdown images blocked for new users
- [ ] Test 4.3: Older users can post images

#### Username Validation

- [ ] Test 5.1: Bad username triggers cleanup
- [ ] Test 5.2: Good username allows registration
- [ ] Test 5.3: Regex patterns work correctly

#### Email Domain Validation

- [ ] Test 6.1: Disposable domains blocked
- [ ] Test 6.2: Custom domain patterns work
- [ ] Test 6.3: Good domains allowed

### Post-Test Cleanup

- [ ] Review test results
- [ ] Document any failures or issues
- [ ] Reset plugin configuration to defaults (optional)
- [ ] Clean test users (optional): `make dev-clean` for complete reset, or delete users via System Console

---

## Additional Notes

- **Timing Considerations:** After modifying user creation dates in the database, allow a few seconds for changes to take effect. The plugin caches user data, so immediate testing may show cached results.

- **Test Isolation:** For best results, test each feature independently. Reset plugin configuration between feature tests if they conflict.

- **Logging:** Always check plugin logs (`make dev-plugin-logs`) if tests fail unexpectedly. Logs may reveal configuration errors or plugin issues.

- **Database Safety:** When modifying user creation dates, always verify changes with a SELECT query before testing. Be careful with UPDATE statements to avoid modifying the wrong users.

- **Automation:** Remember that these are high-level validation tests. Comprehensive testing including edge cases, error handling, and performance should be covered by automated test suites.

---

## Support and Questions

For issues with the development environment, see `docs/PODMAN_DEVELOPMENT.md`.

For plugin development questions, see `CLAUDE.md` and `AGENTS.md`.

For detailed feature documentation, see `docs/NEW_USER_MODERATION.md`.
