#!/bin/sh

PO_HELPER_TEST_REPOSITORY_VERSION=3

# Create test repository in .repository
PO_HELPER_TEST_REPOSITORY="${SHARNESS_TEST_SRCDIR}/test-repository"
PO_HELPER_TEST_REPOSITORY_VERSION_FILE="${PO_HELPER_TEST_REPOSITORY}/.VERSION"

case $(uname) in
Darwin)
	TAR_CMD="tar"
	;;
*)
	TAR_CMD="tar --wildcards"
	;;
esac

cleanup_test_repository_lock () {
       rm -f "${PO_HELPER_TEST_REPOSITORY}.lock"
}

create_test_repository () {
	# create lock
	lockmsg="locked by $$"
	while :
	do
		if test -f "${PO_HELPER_TEST_REPOSITORY}.lock"
		then
			if test "$lockmsg" = "$(cat "${PO_HELPER_TEST_REPOSITORY}.lock")"; then
				break
			fi
			echo >&2 "Another process is creating shared repository: $(cat "${PO_HELPER_TEST_REPOSITORY}.lock")"
			sleep 2

		else
			echo "$lockmsg" >"${PO_HELPER_TEST_REPOSITORY}.lock"
			trap cleanup_test_repository_lock exit
		fi
	done

	if test_repository_is_uptodate
	then
		return
	fi

	# Download git.tgz
	if test ! -f "${SHARNESS_TEST_SRCDIR}/git.tar"
	then
		wget -O "${SHARNESS_TEST_SRCDIR}/git.tar.gz" \
			--progress=dot:mega \
			https://mirrors.edge.kernel.org/pub/software/scm/git/git-2.31.1.tar.gz &&
		gunzip "${SHARNESS_TEST_SRCDIR}/git.tar.gz"
		if test $? -ne 0
		then
			echo >&2 "ERROR: fail to download or unzip git.tar.gz"
			exit 1
		fi
		wget -O "${SHARNESS_TEST_SRCDIR}/git.tar.sign" \
			https://mirrors.edge.kernel.org/pub/software/scm/git/git-2.31.1.tar.sign &&
		gpg --verify "${SHARNESS_TEST_SRCDIR}/git.tar.sign"
		if test $? -ne 0
		then
			echo >&2 "WARNING: cannot verify the signature of the download git package"
		fi
	fi

	# Remove whole shared repository
	if test -d "$PO_HELPER_TEST_REPOSITORY"
	then
		echo >&2 "Will recreate shared repository in $PO_HELPER_TEST_REPOSITORY"
		rm -rf "$PO_HELPER_TEST_REPOSITORY"
	fi

	# Start to create shared repository
	create_test_repository_real

	# create version file
	echo ${PO_HELPER_TEST_REPOSITORY_VERSION} >${PO_HELPER_TEST_REPOSITORY_VERSION_FILE}

	# release the lock
	rm -f "${PO_HELPER_TEST_REPOSITORY}.lock"
}

test_repository_is_uptodate() {
	if test "$(cat "$PO_HELPER_TEST_REPOSITORY_VERSION_FILE" 2>/dev/null)" = "${PO_HELPER_TEST_REPOSITORY_VERSION}"
	then
		return 0
	fi
	return 1
}

create_test_repository_real () {
	git config --global init.defaultbranch master &&
	git init "$PO_HELPER_TEST_REPOSITORY" &&
	${TAR_CMD} --strip-components=1 -C test-repository -xf git.tar -- \
		"git-*/po" \
		"git-*/remote.c" \
		"git-*/wt-status.c" \
		"git-*/builtin/clone.c" \
		"git-*/builtin/checkout.c" \
		"git-*/builtin/index-pack.c" \
		"git-*/builtin/push.c" \
		"git-*/builtin/reset.c"
	(
		cd "$PO_HELPER_TEST_REPOSITORY" &&
		git add -A &&
		git commit -m "Add files from git"
	)
}

# Create test repository
if ! test_repository_is_uptodate
then
	create_test_repository
fi

