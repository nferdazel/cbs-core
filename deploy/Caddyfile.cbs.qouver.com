# Caddy site configuration for cbs.qouver.com & api.qouver.com/cbs
# Append to /etc/caddy/Caddyfile on VPS and reload Caddy: systemctl reload caddy

# 1. API Gateway Handler on api.qouver.com
api.qouver.com {
	handle /cbs/v1/* {
		uri strip_prefix /cbs
		reverse_proxy 127.0.0.1:8082
	}
	handle /cbs/* {
		uri strip_prefix /cbs
		reverse_proxy 127.0.0.1:8082
	}
}

# 2. Backoffice Web Frontend Domain (cbs.qouver.com)
cbs.qouver.com {
	# Proxy API requests directly if called on cbs.qouver.com/api/*
	handle /api/* {
		reverse_proxy 127.0.0.1:8082
	}

	handle /documents/* {
		reverse_proxy 127.0.0.1:8082
	}

	# Serve static web frontend build or proxy to Next.js
	handle {
		root * /srv/qouver/cbs/web
		file_server
		try_files {path} {path}/ /index.html
	}

	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		X-Content-Type-Options "nosniff"
		X-Frame-Options "SAMEORIGIN"
		X-XSS-Protection "1; mode=block"
	}

	encode gzip zstd
}
