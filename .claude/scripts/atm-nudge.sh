#!/usr/bin/env bash
# atm-nudge.sh — post-send hook to nudge Codex agents via tmux
# Called by ATM after sending a message to a member in post_send_hook_members.
# Arguments passed by ATM are currently undocumented; script looks up pane from team config.

set -euo pipefail

TEAM="${ATM_TEAM:-sc-dev}"
CONFIG="$HOME/.claude/teams/$TEAM/config.json"

# Extract recipient name from ATM_POST_SEND env var (JSON, e.g. "csc@sc-dev")
RECIPIENT=$(python3 -c "
import json, os
data = json.loads(os.environ.get('ATM_POST_SEND', '{}'))
to = data.get('to', '')
print(to.split('@')[0] if '@' in to else to)
" 2>/dev/null || true)

if [ -z "$RECIPIENT" ]; then
  exit 0
fi

# Look up tmux pane for recipient
PANE=$(python3 -c "
import json, sys
cfg = json.load(open('$CONFIG'))
member = next((m for m in cfg['members'] if m['name'] == '$RECIPIENT'), None)
if member and member.get('tmuxPaneId'):
    print(member['tmuxPaneId'])
" 2>/dev/null || true)

if [ -z "$PANE" ]; then
  exit 0
fi

# Nudge the pane to read ATM inbox
tmux send-keys -t "$PANE" "atm read --team $TEAM" && sleep 0.5 && tmux send-keys -t "$PANE" "" Enter
