(function() {
    var resetAt = 0;
    var timerId = null;
    var FALLBACK_SECONDS = 60;

    function getBanner() {
        return document.getElementById('rateLimitBanner');
    }

    function getSecondsEl() {
        return document.getElementById('rateLimitSeconds');
    }

    function parseRetryAfter(value) {
        var secs = parseInt(value, 10);
        if (isNaN(secs) || secs < 1) return 0;
        return Math.min(secs, 3600);
    }

    function render() {
        var banner = getBanner();
        if (!banner) return;
        var remaining = Math.max(0, Math.ceil((resetAt - Date.now()) / 1000));
        if (remaining <= 0) {
            hideBanner(banner);
            return;
        }
        var secsEl = getSecondsEl();
        if (secsEl) secsEl.textContent = String(remaining);
        banner.hidden = false;
        requestAnimationFrame(function() {
            banner.classList.add('visible');
        });
    }

    function hideBanner(banner) {
        banner = banner || getBanner();
        if (!banner) return;
        resetAt = 0;
        if (timerId) {
            clearInterval(timerId);
            timerId = null;
        }
        banner.classList.remove('visible');
        banner.hidden = true;
    }

    window.showRateLimit = function(retryAfterSeconds) {
        var secs = parseRetryAfter(retryAfterSeconds);
        if (!secs) secs = FALLBACK_SECONDS;
        resetAt = Math.max(resetAt, Date.now() + secs * 1000);
        render();
        if (!timerId) {
            timerId = setInterval(render, 500);
        }
    };

    window.dismissRateLimit = function() {
        hideBanner();
    };

    function retryAfterFromXHR(xhr) {
        var header = '';
        try {
            header = xhr.getResponseHeader('Retry-After') || '';
        } catch (e) {
            header = '';
        }
        var secs = parseRetryAfter(header);
        if (secs) return secs;
        try {
            var body = JSON.parse(xhr.responseText || '');
            secs = parseRetryAfter(body && body.retry_after);
            if (secs) return secs;
        } catch (e) {}
        return FALLBACK_SECONDS;
    }

    document.addEventListener('htmx:responseError', function(e) {
        var xhr = e && e.detail && e.detail.xhr;
        if (!xhr || xhr.status !== 429) return;
        window.showRateLimit(retryAfterFromXHR(xhr));
    });

    document.addEventListener('htmx:afterSettle', function() {
        if (resetAt > Date.now()) render();
    });

    if (!window.__rateLimitFetchPatched) {
        window.__rateLimitFetchPatched = true;
        var originalFetch = window.fetch.bind(window);
        window.fetch = function() {
            return originalFetch.apply(window, arguments).then(function(res) {
                if (res && res.status === 429) {
                    var header = '';
                    try {
                        header = res.headers.get('Retry-After') || '';
                    } catch (e) {
                        header = '';
                    }
                    var secs = parseRetryAfter(header);
                    if (secs) {
                        window.showRateLimit(secs);
                    } else {
                        try {
                            res.clone().json().then(function(body) {
                                window.showRateLimit(parseRetryAfter(body && body.retry_after) || FALLBACK_SECONDS);
                            }).catch(function() {
                                window.showRateLimit(FALLBACK_SECONDS);
                            });
                        } catch (e) {
                            window.showRateLimit(FALLBACK_SECONDS);
                        }
                    }
                }
                return res;
            });
        };
    }
})();
