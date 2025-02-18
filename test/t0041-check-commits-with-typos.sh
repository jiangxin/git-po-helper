#!/bin/sh

test_description="test git-po-helper check-commits with typos"

. ./lib/sharness.sh

HELPER="git-po-helper --no-gettext-back-compatible"

test_expect_success "setup" '
	git clone "$PO_HELPER_TEST_REPOSITORY" workdir &&
	test -f workdir/po/git.pot &&
	git -C workdir tag -m v1 v1
'

test_expect_success "create po/zh_CN with typos" '
	(
		cd workdir &&
		cat >po/zh_CN.po <<-\EOF &&
		msgid ""
		msgstr ""
		"Project-Id-Version: Git\n"
		"Report-Msgid-Bugs-To: Git Mailing List <git@vger.kernel.org>\n"
		"POT-Creation-Date: 2021-03-04 22:41+0800\n"
		"PO-Revision-Date: 2021-03-04 22:41+0800\n"
		"Last-Translator: Automatically generated\n"
		"Language-Team: none\n"
		"Language: zh_CN\n"
		"MIME-Version: 1.0\n"
		"Content-Type: text/plain; charset=UTF-8\n"
		"Content-Transfer-Encoding: 8bit\n"
		"Plural-Forms: nplurals=2; plural=(n != 1);\n"

		msgid "exit code $res from $command is < 0 or >= 128"
		msgstr "命令的退出码res 应该 < 0 或 >= 128"

		msgid ""
		"Unable to find current ${remote_name}/${branch} revision in submodule path "
		"${sm_path}"
		msgstr ""
		"无法在子模块路径 sm_path 中找到当前的 远程/分支 版本"
		EOF

		git add "po/zh_CN.po" &&
		test_tick &&
		git commit -s -m "l10n: add po/zh_CN" &&
		git tag -m v2 v2
	)
'

cat >workdir/expect <<-\EOF
level=info msg="[po/zh_CN.po@4e9d487]    2 translated messages."
level=warning msg="[po/zh_CN.po@4e9d487]    mismatch variable names: $branch, $remote_name, $sm_path, sm_path"
level=warning msg="[po/zh_CN.po@4e9d487]    >> msgid: Unable to find current ${remote_name}/${branch} revision in submodule path ${sm_path}"
level=warning msg="[po/zh_CN.po@4e9d487]    >> msgstr: 无法在子模块路径 sm_path 中找到当前的 远程/分支 版本"
level=warning
level=warning msg="[po/zh_CN.po@4e9d487]    mismatch variable names: $command, $res"
level=warning msg="[po/zh_CN.po@4e9d487]    >> msgid: exit code $res from $command is < 0 or >= 128"
level=warning msg="[po/zh_CN.po@4e9d487]    >> msgstr: 命令的退出码res 应该 < 0 或 >= 128"
level=warning
level=warning msg="commit <OID>: author (A U Thor <author@example.com>) and committer (C O Mitter <committer@example.com>) are different"
level=info msg="checking commits: 1 passed."
EOF

test_expect_success "check-commits show typos" '
	(
		cd workdir &&
		$HELPER check-commits v1.. >out 2>&1 &&
		make_user_friendly_and_stable_output <out >actual &&
		test_cmp expect actual
	)
'

cat >workdir/expect <<-\EOF
level=info msg="[po/zh_CN.po@4e9d487]    2 translated messages."
level=error msg="[po/zh_CN.po@4e9d487]    mismatch variable names: $branch, $remote_name, $sm_path, sm_path"
level=error msg="[po/zh_CN.po@4e9d487]    >> msgid: Unable to find current ${remote_name}/${branch} revision in submodule path ${sm_path}"
level=error msg="[po/zh_CN.po@4e9d487]    >> msgstr: 无法在子模块路径 sm_path 中找到当前的 远程/分支 版本"
level=error
level=error msg="[po/zh_CN.po@4e9d487]    mismatch variable names: $command, $res"
level=error msg="[po/zh_CN.po@4e9d487]    >> msgid: exit code $res from $command is < 0 or >= 128"
level=error msg="[po/zh_CN.po@4e9d487]    >> msgstr: 命令的退出码res 应该 < 0 或 >= 128"
level=error
level=warning msg="commit <OID>: author (A U Thor <author@example.com>) and committer (C O Mitter <committer@example.com>) are different"
level=info msg="checking commits: 0 passed, 1 failed."

ERROR: fail to execute "git-po-helper check-commits"
EOF

test_expect_success "check-commits show typos (--report-typos-as-errors)" '
	(
		cd workdir &&
		test_must_fail $HELPER check-commits --report-typos-as-errors v1.. >out 2>&1 &&
		make_user_friendly_and_stable_output <out >actual &&
		test_cmp expect actual
	)
'

test_expect_success "update po/TEAMS" '
	(
		cd workdir &&
		echo >>po/TEAMS &&
		git add -u &&
		test_tick &&
		git commit -s -m "l10n: TEAMS: update for test"
	)
'

cat >workdir/expect <<-\EOF
level=error msg="commit <OID>: bad syntax at line 79 (unknown key \"Respository\"): Respository:    https://github.com/l10n-tw/git-po"
level=error msg="commit <OID>: bad syntax at line 80 (need two tabs between k/v): Leader:     Yi-Jyun Pan <pan93412 AT gmail.com>"
level=warning msg="commit <OID>: author (A U Thor <author@example.com>) and committer (C O Mitter <committer@example.com>) are different"
level=info msg="[po/zh_CN.po@4e9d487]    2 translated messages."
level=warning msg="[po/zh_CN.po@4e9d487]    mismatch variable names: $branch, $remote_name, $sm_path, sm_path"
level=warning msg="[po/zh_CN.po@4e9d487]    >> msgid: Unable to find current ${remote_name}/${branch} revision in submodule path ${sm_path}"
level=warning msg="[po/zh_CN.po@4e9d487]    >> msgstr: 无法在子模块路径 sm_path 中找到当前的 远程/分支 版本"
level=warning
level=warning msg="[po/zh_CN.po@4e9d487]    mismatch variable names: $command, $res"
level=warning msg="[po/zh_CN.po@4e9d487]    >> msgid: exit code $res from $command is < 0 or >= 128"
level=warning msg="[po/zh_CN.po@4e9d487]    >> msgstr: 命令的退出码res 应该 < 0 或 >= 128"
level=warning
level=warning msg="commit <OID>: author (A U Thor <author@example.com>) and committer (C O Mitter <committer@example.com>) are different"
level=info msg="checking commits: 1 passed, 1 failed."

ERROR: fail to execute "git-po-helper check-commits"
EOF

test_expect_success "check-commits show typos and TEAMS file" '
	(
		cd workdir &&
		test_must_fail $HELPER check-commits v1.. >out 2>&1 &&
		make_user_friendly_and_stable_output <out >actual &&
		test_cmp expect actual
	)
'

test_done
