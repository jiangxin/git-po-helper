#!/bin/sh

test_description="test git-po-helper agent-run translate"

. ./lib/test-lib.sh

HELPER="po-helper --no-special-gettext-versions"

# Create a mock agent script that simulates translation
create_mock_translate_agent() {
	cat >"$1" <<'EOF'
#!/bin/sh
# Mock agent that translates a PO file by filling in msgstr entries
# Usage: mock-translate-agent --prompt "<prompt>" <source>

# Parse arguments
SOURCE=""
while [ $# -gt 0 ]; do
	case "$1" in
		--prompt|-p)
			shift
			PROMPT="$1"
			;;
		*)
			# Treat the first non-flag argument as source path
			if [ -z "$SOURCE" ]; then
				SOURCE="$1"
			fi
			;;
	esac
	shift
done

if [ -z "$SOURCE" ] || [ ! -f "$SOURCE" ]; then
	echo "Error: PO file not found: $SOURCE" >&2
	exit 1
fi

# Use msgattrib to get untranslated entries
TEMP_UNTRANSLATED=$(mktemp)
msgattrib --untranslated "$SOURCE" > "$TEMP_UNTRANSLATED" 2>/dev/null || {
	echo "Error: msgattrib failed for untranslated entries" >&2
	rm -f "$TEMP_UNTRANSLATED"
	exit 1
}

# Use msgattrib to get fuzzy entries
TEMP_FUZZY=$(mktemp)
msgattrib --only-fuzzy "$SOURCE" > "$TEMP_FUZZY" 2>/dev/null || {
	echo "Error: msgattrib failed for fuzzy entries" >&2
	rm -f "$TEMP_UNTRANSLATED" "$TEMP_FUZZY"
	exit 1
}

# Create a temporary file for processing
TEMP_OUTPUT=$(mktemp)
cp "$SOURCE" "$TEMP_OUTPUT"

# Process the PO file to fill in translations
awk '
BEGIN { in_msgid = 0; in_msgstr = 0; msgid_value = ""; is_fuzzy = 0 }

# Check for fuzzy flag
/^#,.*fuzzy/ { is_fuzzy = 1; next }

# Check for msgid
/^msgid / {
	in_msgid = 1
	msgid_value = $0
	next
}

# Check for msgstr
/^msgstr / {
	in_msgid = 0
	in_msgstr = 1
	# If msgstr is empty and we have a non-empty msgid, fill it
	if ($0 ~ /^msgstr ""$/ && msgid_value !~ /^msgid ""$/) {
		print "msgstr \"[TRANSLATED]" substr(msgid_value, 8) "\""
		next
	}
	# If this is a fuzzy entry, remove the fuzzy flag and update msgstr
	if (is_fuzzy && $0 !~ /^msgstr ""$/) {
		# Output the line, but mark it as updated
		print $0
		is_fuzzy = 0
		next
	}
	print
	next
}

# Skip fuzzy flags
/^#,/ {
	if (!is_fuzzy) {
		print
	} else {
		# Remove fuzzy flag
		gsub(/,?[ \t]*fuzzy[ \t]*,?/, "", $0)
		if ($0 !~ /^#,[ \t]*$/) {
			print
		}
		is_fuzzy = 0
	}
	next
}

# Print all other lines
{ print }
' "$TEMP_OUTPUT" > "$SOURCE"

rm -f "$TEMP_UNTRANSLATED" "$TEMP_FUZZY" "$TEMP_OUTPUT"
exit 0
EOF
	chmod +x "$1"
}

test_expect_success "setup" '
	git clone "$PO_HELPER_TEST_REPOSITORY" workdir &&
	test -f workdir/po/git.pot &&
	test -f workdir/po/zh_CN.po &&
	# Create mock translate agent
	create_mock_translate_agent "$PWD/mock-translate-agent" &&
	# Create config file
	cat >workdir/git-po-helper.yaml <<-\EOF &&
default_lang_code: "zh_CN"
prompt:
  translate: "translate {source} according to po/README.md"
agents:
  mock:
    cmd: ["$PWD/mock-translate-agent", "--prompt", "{prompt}", "{source}"]
EOF
	# Replace $PWD with actual path in config
	sed -i.bak "s|\$PWD|$PWD|g" workdir/git-po-helper.yaml &&
	rm -f workdir/git-po-helper.yaml.bak
'

test_expect_success "agent-run translate: no config file" '
	rm -f workdir/git-po-helper.yaml &&
	test_must_fail git -C workdir $HELPER agent-run translate >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	cat >expect <<-\EOF &&
	level=error msg="failed to load agent configuration: no configuration files found"

	ERROR: fail to execute "git-po-helper agent-run translate"
	EOF

	test_cmp expect actual
'

test_expect_success "agent-run translate: restore config" '
	cat >workdir/git-po-helper.yaml <<-\EOF &&
default_lang_code: "zh_CN"
prompt:
  translate: "translate {source} according to po/README.md"
agents:
  mock:
    cmd: ["$PWD/mock-translate-agent", "--prompt", "{prompt}", "{source}"]
EOF
	sed -i.bak "s|\$PWD|$PWD|g" workdir/git-po-helper.yaml &&
	rm -f workdir/git-po-helper.yaml.bak
'

test_expect_success "agent-run translate: missing prompt.translate" '
	cat >workdir/git-po-helper.yaml <<-\EOF &&
default_lang_code: "zh_CN"
prompt:
  update_pot: "update po/git.pot"
agents:
  mock:
    cmd: ["$PWD/mock-translate-agent", "--prompt", "{prompt}", "{source}"]
EOF
	sed -i.bak "s|\$PWD|$PWD|g" workdir/git-po-helper.yaml &&
	rm -f workdir/git-po-helper.yaml.bak &&

	test_must_fail git -C workdir $HELPER agent-run translate >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	grep "prompt.translate is not configured" actual
'

test_expect_success "agent-run translate: restore config" '
	cat >workdir/git-po-helper.yaml <<-\EOF &&
default_lang_code: "zh_CN"
prompt:
  translate: "translate {source} according to po/README.md"
agents:
  mock:
    cmd: ["$PWD/mock-translate-agent", "--prompt", "{prompt}", "{source}"]
EOF
	sed -i.bak "s|\$PWD|$PWD|g" workdir/git-po-helper.yaml &&
	rm -f workdir/git-po-helper.yaml.bak
'

test_expect_success "agent-run translate: PO file does not exist" '
	test_must_fail git -C workdir $HELPER agent-run translate po/nonexistent.po >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	grep "PO file does not exist" actual
'

test_expect_success "agent-run translate: success with default_lang_code" '
	# Create a PO file with untranslated entries
	msgmerge -q --no-fuzzy-matching -o workdir/po/zh_CN.po \
		workdir/po/zh_CN.po workdir/po/git.pot &&

	# Count new entries before translation
	NEW_BEFORE=$(msgattrib --untranslated workdir/po/zh_CN.po | grep -c "^msgid " || true) &&
	test "$NEW_BEFORE" -gt 0 &&

	git -C workdir $HELPER agent-run translate >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	# Should complete successfully
	grep "completed successfully" actual &&

	# Count new entries after translation (should be 0)
	NEW_AFTER=$(msgattrib --untranslated workdir/po/zh_CN.po | grep -c "^msgid " || true) &&
	test "$NEW_AFTER" -eq 1
'

test_expect_success "agent-run translate: success with explicit PO file" '
	# Create a PO file with untranslated entries
	msgmerge -q --no-fuzzy-matching -o workdir/po/zh_CN.po \
		workdir/po/zh_CN.po workdir/po/git.pot &&

	# Count new entries before translation
	NEW_BEFORE=$(msgattrib --untranslated workdir/po/zh_CN.po | grep -c "^msgid " || true) &&
	test "$NEW_BEFORE" -gt 0 &&

	git -C workdir $HELPER agent-run translate po/zh_CN.po >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	# Should complete successfully
	grep "completed successfully" actual &&

	# Count new entries after translation (should be 0)
	NEW_AFTER=$(msgattrib --untranslated workdir/po/zh_CN.po | grep -c "^msgid " || true) &&
	test "$NEW_AFTER" -eq 1
'

test_expect_success "agent-run translate: agent command failure" '
	# Create a failing mock agent
	cat >"$PWD/failing-translate-agent" <<-\EOF &&
	#!/bin/sh
	echo "Translation agent failed" >&2
	exit 1
	EOF
	chmod +x "$PWD/failing-translate-agent" &&

	cat >workdir/git-po-helper.yaml <<-EOF &&
default_lang_code: "zh_CN"
prompt:
  translate: "translate {source} according to po/README.md"
agents:
  failing:
    cmd: ["$PWD/failing-translate-agent", "--prompt", "{prompt}", "{source}"]
EOF
	sed -i.bak "s|\$PWD|$PWD|g" workdir/git-po-helper.yaml &&
	rm -f workdir/git-po-helper.yaml.bak &&

	test_must_fail git -C workdir $HELPER agent-run translate >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	# Should mention agent command failure
	grep "agent command failed" actual
'

test_expect_success "agent-run translate: no new or fuzzy entries" '
	# Restore config
	cat >workdir/git-po-helper.yaml <<-\EOF &&
default_lang_code: "zh_CN"
prompt:
  translate: "translate {source} according to po/README.md"
agents:
  mock:
    cmd: ["$PWD/mock-translate-agent", "--prompt", "{prompt}", "{source}"]
EOF
	sed -i.bak "s|\$PWD|$PWD|g" workdir/git-po-helper.yaml &&
	rm -f workdir/git-po-helper.yaml.bak &&

	# Ensure PO file has no new or fuzzy entries
	git -C workdir $HELPER agent-run translate po/zh_CN.po >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	# Should indicate no work needed
	grep "no new or fuzzy entries to translate" actual
'

test_done
