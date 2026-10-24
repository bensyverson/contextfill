#!/bin/sh
# Prints the shape of OpenCode's local session store, read-only: the key
# paths of the newest assistant message, and per provider how many assistant
# steps lack tokens or carry none on the input side. Prints no message
# content. Usage: scripts/opencode-shape.sh [path/to/opencode.db]
set -eu
db="${1:-${OPENCODE_DB:-${XDG_DATA_HOME:-$HOME/.local/share}/opencode/opencode.db}}"
q() { sqlite3 -header "file:$db?mode=ro" "$1"; }

echo "== key paths of the newest assistant message"
q "select distinct j.fullkey path, j.type from
     (select data from session_message where type = 'assistant' order by time_created desc limit 1) m,
     json_tree(m.data) j
   where j.type not in ('object', 'array') order by 1;" | sed -E 's/\[[0-9]+\]/[n]/g' | uniq

echo "== assistant steps per provider"
q "with t as (select json_extract(data, '$.model.providerID') provider,
       json_type(data, '$.tokens') is null no_tokens,
       coalesce(json_extract(data, '$.tokens.input') + json_extract(data, '$.tokens.cache.read')
         + json_extract(data, '$.tokens.cache.write'), 0) = 0 zero_input,
       json_type(data, '$.error') is not null errored
     from session_message where type = 'assistant')
   select provider, count(*) steps, sum(no_tokens) no_tokens, sum(zero_input) zero_input,
     sum(zero_input and not errored) zero_input_without_error
   from t group by provider;"

echo "== sessions"
q "select count(*) sessions, sum(parent_id is not null) subagents from session_v2;"
