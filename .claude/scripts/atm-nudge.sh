#!/usr/bin/env bash
# atm-nudge.sh — post-send hook to nudge Codex agents via tmux
# Called by ATM after sending a message to a member in post_send_hook_members.
# Arguments passed by ATM are currently undocumented; script looks up pane from team config.

set -euo pipefail

TEAM="${ATM_TEAM:-sc-dev}"
CONFIG="$HOME/.claude/teams/$TEAM/config.json"

# Debug log
echo "[$(date -u +%FT%TZ)] args=$* ATM_POST_SEND=${ATM_POST_SEND:-UNSET}" >> /tmp/atm-nudge-debug.log

# $1 = recipient; task_id optionally in ATM_POST_SEND JSON
RECIPIENT="${1:-}"
TASK_ID="$(printf '%s' "${ATM_POST_SEND:-}" | jq -r '.task_id // empty' 2>/dev/null || true)"

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

# Nudge: inject ATM action directive into csc's pane
ACK_ACTION=""
if [ -n "$TASK_ID" ]; then
  ACK_ACTION="<action>ack ${TASK_ID}</action>"
fi

NUDGE="<atm><action>read atm</action>${ACK_ACTION}<action>execute assigned task</action><when idle=\"immediate\" busy=\"after-current-task\"/><console announce=\"concise\" pause=\"false\"/></atm>"
echo "[$(date -u +%FT%TZ)] pane=$PANE nudge=$NUDGE" >> /tmp/atm-nudge-debug.log

sleep 0.5
tmux set-buffer "$NUDGE"
tmux paste-buffer -t "$PANE"
sleep 0.25
tmux send-keys -t "$PANE" "" Enter
