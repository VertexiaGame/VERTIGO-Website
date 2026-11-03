// Adapted from https://github.com/tylxr59/oneko.js (based on adryd325/oneko.js)
(function() {
    var SPRITE = '/static/useful/oneko.gif';
    var SPEED = 10;
    var SIZE = 32;

    var spriteSets = {
        idle: [[-3, -3]],
        alert: [[-7, -3]],
        scratchSelf: [[-5, 0], [-6, 0], [-7, 0]],
        scratchWallN: [[0, 0], [0, -1]],
        scratchWallS: [[-7, -1], [-6, -2]],
        scratchWallE: [[-2, -2], [-2, -3]],
        scratchWallW: [[-4, 0], [-4, -1]],
        tired: [[-3, -2]],
        sleeping: [[-2, 0], [-2, -1]],
        N: [[-1, -2], [-1, -3]],
        NE: [[0, -2], [0, -3]],
        E: [[-3, 0], [-3, -1]],
        SE: [[-5, -1], [-5, -2]],
        S: [[-6, -3], [-7, -2]],
        SW: [[-5, -3], [-6, -1]],
        W: [[-4, -2], [-4, -3]],
        NW: [[-1, 0], [-1, -1]]
    };

    var active = null;

    var reducedMotion = function() {
        return window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    };

    var ensureStyle = function() {
        if (document.getElementById('onekoStyle')) return;
        var style = document.createElement('style');
        style.id = 'onekoStyle';
        style.textContent = '@keyframes onekoHeartBurst{0%{transform:scale(0);opacity:1}100%{transform:scale(1);opacity:0}}' +
            '.oneko-heart{position:fixed;font-size:2em;line-height:1;pointer-events:none;animation:onekoHeartBurst 1s ease-out forwards;color:#ab9df2;}';
        document.head.appendChild(style);
    };

    var explodeHearts = function(el) {
        var parent = el.parentElement;
        if (!parent) return;
        var rect = el.getBoundingClientRect();
        var centerX = rect.left + rect.width / 2;
        var centerY = rect.top + rect.height / 2;
        for (var i = 0; i < 10; i++) {
            var heart = document.createElement('div');
            heart.className = 'oneko-heart';
            heart.textContent = '\u2764';
            var offsetX = (Math.random() - 0.5) * 50;
            var offsetY = (Math.random() - 0.5) * 50;
            heart.style.left = (centerX + offsetX - 16) + 'px';
            heart.style.top = (centerY + offsetY - 16) + 'px';
            heart.style.transform = 'translate(-50%, -50%) rotate(' + (Math.random() * 360) + 'deg)';
            parent.appendChild(heart);
            window.setTimeout(function(node) {
                return function() {
                    if (node.parentElement) node.parentElement.removeChild(node);
                };
            }(heart), 1000);
        }
    };

    var start = function() {
        if (active) return;
        if (!document.body) {
            document.addEventListener('DOMContentLoaded', function() { window.startOneko(); }, { once: true });
            return;
        }
        if (reducedMotion()) return;

        ensureStyle();

        var el = document.createElement('div');
        el.id = 'oneko';
        el.setAttribute('aria-hidden', 'true');
        el.style.width = SIZE + 'px';
        el.style.height = SIZE + 'px';
        el.style.position = 'fixed';
        el.style.pointerEvents = 'auto';
        el.style.imageRendering = 'pixelated';
        el.style.zIndex = '2147483647';
        el.style.backgroundImage = 'url(' + SPRITE + ')';
        el.style.cursor = 'pointer';

        var posX = 32;
        var posY = 32;
        var mouseX = 0;
        var mouseY = 0;
        var frameCount = 0;
        var idleTime = 0;
        var idleAnimation = null;
        var idleAnimationFrame = 0;
        var lastFrameTimestamp;
        var state = { el: el, onMouseMove: null, raf: 0 };

        var setSprite = function(name, frame) {
            var set = spriteSets[name] || spriteSets.idle;
            var sprite = set[frame % set.length];
            el.style.backgroundPosition = (sprite[0] * SIZE) + 'px ' + (sprite[1] * SIZE) + 'px';
        };

        var resetIdleAnimation = function() {
            idleAnimation = null;
            idleAnimationFrame = 0;
        };

        var idle = function() {
            idleTime += 1;
            if (idleTime > 10 && Math.floor(Math.random() * 200) === 0 && idleAnimation === null) {
                var options = ['sleeping', 'scratchSelf'];
                if (posX < SIZE) options.push('scratchWallW');
                if (posY < SIZE) options.push('scratchWallN');
                if (posX > window.innerWidth - SIZE) options.push('scratchWallE');
                if (posY > window.innerHeight - SIZE) options.push('scratchWallS');
                idleAnimation = options[Math.floor(Math.random() * options.length)];
            }
            switch (idleAnimation) {
                case 'sleeping':
                    if (idleAnimationFrame < 8) {
                        setSprite('tired', 0);
                        break;
                    }
                    setSprite('sleeping', Math.floor(idleAnimationFrame / 4));
                    if (idleAnimationFrame > 192) resetIdleAnimation();
                    break;
                case 'scratchWallN':
                case 'scratchWallS':
                case 'scratchWallE':
                case 'scratchWallW':
                case 'scratchSelf':
                    setSprite(idleAnimation, idleAnimationFrame);
                    if (idleAnimationFrame > 9) resetIdleAnimation();
                    break;
                default:
                    setSprite('idle', 0);
                    return;
            }
            idleAnimationFrame += 1;
        };

        var frame = function() {
            frameCount += 1;
            var diffX = posX - mouseX;
            var diffY = posY - mouseY;
            var distance = Math.sqrt(diffX * diffX + diffY * diffY);
            if (distance < SPEED || distance < 48) {
                idle();
                return;
            }
            idleAnimation = null;
            idleAnimationFrame = 0;
            if (idleTime > 1) {
                setSprite('alert', 0);
                idleTime = Math.min(idleTime, 7);
                idleTime -= 1;
                return;
            }
            var direction = diffY / distance > 0.5 ? 'N' : '';
            direction += diffY / distance < -0.5 ? 'S' : '';
            direction += diffX / distance > 0.5 ? 'W' : '';
            direction += diffX / distance < -0.5 ? 'E' : '';
            setSprite(direction, frameCount);
            posX -= (diffX / distance) * SPEED;
            posY -= (diffY / distance) * SPEED;
            posX = Math.min(Math.max(16, posX), window.innerWidth - 16);
            posY = Math.min(Math.max(16, posY), window.innerHeight - 16);
            el.style.left = (posX - 16) + 'px';
            el.style.top = (posY - 16) + 'px';
        };

        var onAnimationFrame = function(timestamp) {
            if (!el.isConnected) return;
            if (!lastFrameTimestamp) lastFrameTimestamp = timestamp;
            if (timestamp - lastFrameTimestamp > 100) {
                lastFrameTimestamp = timestamp;
                frame();
            }
            state.raf = window.requestAnimationFrame(onAnimationFrame);
        };

        var onMouseMove = function(event) {
            mouseX = event.clientX;
            mouseY = event.clientY;
        };

        el.style.left = (posX - 16) + 'px';
        el.style.top = (posY - 16) + 'px';
        el.addEventListener('click', function() { explodeHearts(el); });
        document.addEventListener('mousemove', onMouseMove, { passive: true });
        document.body.appendChild(el);

        state.onMouseMove = onMouseMove;
        active = state;
        state.raf = window.requestAnimationFrame(onAnimationFrame);
    };

    var stop = function() {
        if (!active) return;
        if (active.raf) window.cancelAnimationFrame(active.raf);
        document.removeEventListener('mousemove', active.onMouseMove);
        if (active.el && active.el.parentElement) active.el.parentElement.removeChild(active.el);
        active = null;
        document.documentElement.removeAttribute('data-oneko');
    };

    window.startOneko = function() {
        document.documentElement.setAttribute('data-oneko', '1');
        start();
    };

    window.stopOneko = function() {
        stop();
    };

    window.setOneko = function(enabled) {
        if (enabled) {
            window.startOneko();
        } else {
            window.stopOneko();
        }
    };

    window.isOnekoActive = function() {
        return !!active;
    };

    var ensureRunning = function() {
        if (!document.documentElement.hasAttribute('data-oneko')) return;
        if (active && active.el && active.el.isConnected) return;
        if (active) {
            if (active.raf) window.cancelAnimationFrame(active.raf);
            document.removeEventListener('mousemove', active.onMouseMove);
            active = null;
        }
        start();
    };

    document.addEventListener('htmx:afterSettle', ensureRunning);
    document.addEventListener('htmx:afterSwap', ensureRunning);

    if (document.documentElement.hasAttribute('data-oneko')) {
        start();
    }
})();
