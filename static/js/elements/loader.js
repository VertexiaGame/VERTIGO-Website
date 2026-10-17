(function() {
    let pending = 0;

    window.navigateWithHistory = function(url) {
        if (!url) return null;
        if (!window.htmx) {
            window.location.href = url;
            return null;
        }
        var src = document.getElementById('glbHxNavSrc');
        if (!src) {
            src = document.createElement('span');
            src.id = 'glbHxNavSrc';
            src.style.display = 'none';
            src.setAttribute('aria-hidden', 'true');
            document.body.appendChild(src);
        }
        src.setAttribute('hx-push-url', 'true');
        return window.htmx.ajax('GET', url, {source: src, target: 'body'});
    };

    function getLoader() {
        return document.getElementById('globalLoader');
    }

    function getProgress() {
        return document.getElementById('pageProgress');
    }

    function showLoader() {
        const el = getLoader();
        if (el) {
            el.classList.add('visible');
            el.style.display = 'block';
            el.setAttribute('aria-hidden', 'false');
        }
        const bar = getProgress();
        if (bar) {
            bar.classList.add('active');
            bar.setAttribute('aria-hidden', 'false');
        }
    }

    function hideLoader() {
        if (pending > 0) return;
        const el = getLoader();
        if (el) {
            el.classList.remove('visible');
            el.style.display = 'none';
            el.setAttribute('aria-hidden', 'true');
        }
        const bar = getProgress();
        if (bar) {
            bar.classList.remove('active');
            bar.setAttribute('aria-hidden', 'true');
        }
    }

    function inc() {
        pending++;
        showLoader();
    }

    function dec() {
        pending = Math.max(0, pending - 1);
        if (pending === 0) {
            hideLoader();
        }
    }

    function isPageLoad(e) {
        if (!e || !e.detail) return true;
        if (e.detail.boosted) return true;
        if (e.detail.requestConfig && e.detail.requestConfig.boosted) return true;
        const elt = e.detail.elt;
        if (!elt) return false;
        if (elt.closest && elt.closest('[hx-boost]')) return true;
        if (elt.getAttribute) {
            if (elt.getAttribute('hx-push-url') === 'true') return true;
            if (elt.getAttribute('hx-target') === 'body') return true;
        }
        return false;
    }

    document.addEventListener('htmx:beforeRequest', function(e) {
        if (isPageLoad(e)) inc();
    });
    document.addEventListener('htmx:afterRequest', function(e) {
        if (isPageLoad(e)) setTimeout(dec, 80);
    });
    document.addEventListener('htmx:responseError', function(e) {
        if (isPageLoad(e)) dec();
    });
    document.addEventListener('htmx:sendError', function(e) {
        if (isPageLoad(e)) dec();
    });
    document.addEventListener('htmx:afterSettle', function() {
        pending = 0;
        hideLoader();
    });
    document.addEventListener('htmx:beforeTransition', function() {
        inc();
    });
    var unloadTimer = null;
    function clearUnloadTimer() {
        if (unloadTimer) {
            clearTimeout(unloadTimer);
            unloadTimer = null;
        }
    }
    function forceHideLoader() {
        pending = 0;
        clearUnloadTimer();
        hideLoader();
    }
    function onBeforeUnload() {
        showLoader();
        clearUnloadTimer();
        unloadTimer = setTimeout(function() {
            unloadTimer = null;
            if (document.visibilityState !== 'hidden') {
                pending = 0;
                hideLoader();
            }
        }, 1200);
    }
    window.hideGlobalLoader = forceHideLoader;
    window.resetGlobalLoader = forceHideLoader;
    document.addEventListener('visibilitychange', function() {
        if (document.visibilityState === 'visible' && pending === 0) {
            hideLoader();
        }
    });
    window.addEventListener('pagehide', clearUnloadTimer);
    window.addEventListener('beforeunload', onBeforeUnload);
    window.addEventListener('load', function() {
        pending = 0;
        clearUnloadTimer();
        hideLoader();
    });
    document.addEventListener('DOMContentLoaded', hideLoader);
    hideLoader();
})();
