#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

allowed_direct_modules='github.com/evanw/esbuild
golang.org/x/tools'

optional_modules='github.com/cssbruno/gowdk/runtime/adapters/chi
github.com/cssbruno/gowdk/runtime/adapters/echo
github.com/cssbruno/gowdk/runtime/adapters/fiber
github.com/cssbruno/gowdk/runtime/adapters/gin
github.com/cssbruno/gowdk/runtime/contracts/natsbroker
github.com/cssbruno/gowdk/runtime/contracts/redisstream
github.com/cssbruno/gowdk/runtime/contracts/websocketfanout
github.com/coder/websocket
github.com/gin-gonic/gin
github.com/go-chi/chi/v5
github.com/gofiber/fiber/v2
github.com/labstack/echo/v5
github.com/nats-io/nats.go
github.com/redis/go-redis/v9'

actual_direct_modules=$(
	cd "${repo_root}" &&
		GOWORK=off go list -m -f '{{if and (not .Main) (not .Indirect)}}{{.Path}}{{end}}' all |
		sed '/^$/d' |
		sort
)

expected_direct_modules=$(printf '%s\n' "${allowed_direct_modules}" | sort)

if [ "${actual_direct_modules}" != "${expected_direct_modules}" ]; then
	printf '%s\n' "root go.mod direct dependency surface changed." >&2
	printf '%s\n' "" >&2
	printf '%s\n' "Expected direct modules:" >&2
	printf '%s\n' "${expected_direct_modules}" >&2
	printf '%s\n' "" >&2
	printf '%s\n' "Actual direct modules:" >&2
	printf '%s\n' "${actual_direct_modules}" >&2
	printf '%s\n' "" >&2
	printf '%s\n' "Move optional integrations to nested modules, or update scripts/check-root-deps.sh and docs/engineering/dependency-policy.md with the rationale." >&2
	exit 1
fi

root_modules=$(
	cd "${repo_root}" &&
		GOWORK=off go list -m all |
		awk '{print $1}' |
		sort
)

unexpected_optional_modules=""
for module in ${optional_modules}; do
	if printf '%s\n' "${root_modules}" | grep -Fx "${module}" >/dev/null; then
		unexpected_optional_modules="${unexpected_optional_modules}
${module}"
	fi
done

if [ -n "${unexpected_optional_modules}" ]; then
	printf '%s\n' "root module graph includes optional adapter dependencies:" >&2
	printf '%s\n' "${unexpected_optional_modules}" | sed '/^$/d' >&2
	printf '%s\n' "" >&2
	printf '%s\n' "Keep framework, broker, realtime, and Tailwind/npm-style integrations outside the root module graph." >&2
	exit 1
fi

printf '%s\n' "root dependency surface ok"
