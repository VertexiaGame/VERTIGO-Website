(function() {
    var DIM = 'g21-dim';
    var TIP = 'g21Tip';
    var DELAY = 1200;
    var timer = null;
    var lastX = 0;
    var lastY = 0;
    var holdX = 0;
    var holdY = 0;

    function ensureTip() {
        var tip = document.getElementById(TIP);
        if (!tip) {
            tip = document.createElement('div');
            tip.id = TIP;
            tip.textContent = 'Made by G21!';
            tip.setAttribute('role', 'tooltip');
            tip.setAttribute('aria-hidden', 'true');
            document.body.appendChild(tip);
        }
        return tip;
    }

    function placeTip(x, y) {
        var tip = ensureTip();
        var w = tip.offsetWidth || 110;
        var h = tip.offsetHeight || 30;
        var left = x + 14;
        var top = y + 18;
        if (left + w > window.innerWidth - 8) left = x - w - 12;
        if (top + h > window.innerHeight - 8) top = y - h - 12;
        if (left < 8) left = 8;
        if (top < 8) top = 8;
        tip.style.left = left + 'px';
        tip.style.top = top + 'px';
    }

    function clearDim() {
        if (timer) {
            clearTimeout(timer);
            timer = null;
        }
        if (document.body) document.body.classList.remove(DIM);
        var tip = document.getElementById(TIP);
        if (tip) {
            tip.classList.remove('visible');
            tip.setAttribute('aria-hidden', 'true');
        }
    }

    function applyDim() {
        timer = null;
        if (document.querySelector('.movl.active')) return;
        if (document.body) document.body.classList.add(DIM);
        var tip = ensureTip();
        placeTip(lastX, lastY);
        tip.classList.add('visible');
        tip.setAttribute('aria-hidden', 'false');
    }

    function isBackground(el) {
        if (!el) return false;
        if (el === document.body || el === document.documentElement) return true;
        if (el.classList && el.classList.contains('wrap')) return true;
        return false;
    }

    document.addEventListener('mousemove', function(e) {
        lastX = e.clientX;
        lastY = e.clientY;
        var t = e.target;
        if (t && t.nodeType !== 1) t = t.parentElement;
        if (isBackground(t)) {
            if (document.body && document.body.classList.contains(DIM)) {
                placeTip(lastX, lastY);
            } else if (!timer) {
                timer = setTimeout(applyDim, DELAY);
            }
        } else {
            clearDim();
        }
    }, {passive: true});

    document.addEventListener('mouseleave', clearDim);
    document.addEventListener('scroll', clearDim, {capture: true, passive: true});
    document.addEventListener('keydown', clearDim);
    document.addEventListener('mousedown', clearDim);
    document.addEventListener('touchstart', function(e) {
        var t = e.target;
        if (t && t.nodeType !== 1) t = t.parentElement;
        if (isBackground(t)) {
            var touch = e.touches && e.touches[0];
            if (touch) {
                lastX = touch.clientX;
                lastY = touch.clientY;
                holdX = touch.clientX;
                holdY = touch.clientY;
            }
            if (!timer && !(document.body && document.body.classList.contains(DIM))) {
                timer = setTimeout(applyDim, DELAY);
            }
        } else {
            clearDim();
        }
    }, {passive: true});
    document.addEventListener('touchmove', function(e) {
        var touch = e.touches && e.touches[0];
        if (touch && Math.abs(touch.clientX - holdX) + Math.abs(touch.clientY - holdY) > 12) {
            if (timer) {
                clearTimeout(timer);
                timer = null;
            }
        }
    }, {passive: true});
    document.addEventListener('touchend', function() {
        if (timer) {
            clearTimeout(timer);
            timer = null;
        }
    });
    document.addEventListener('touchcancel', function() {
        if (timer) {
            clearTimeout(timer);
            timer = null;
        }
    });
    document.addEventListener('visibilitychange', function() {
        if (document.hidden) clearDim();
    });
    document.addEventListener('htmx:afterSettle', clearDim);
    document.addEventListener('htmx:beforeTransition', clearDim);
})();
