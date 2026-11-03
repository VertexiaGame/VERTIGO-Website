(function() {
    window.clanEsc = function(str) {
        if (window.escapeHTML) return window.escapeHTML(String(str == null ? '' : str));
        return String(str == null ? '' : str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
    };

    window.clanCSRFToken = function() {
        var node = document.querySelector('[data-csrf]');
        if (node && node.dataset.csrf) return node.dataset.csrf;
        var input = document.querySelector('input[name="csrf"]');
        return input ? input.value : '';
    };

    window.clanPost = async function(url, options) {
        options = options || {};
        var button = options.button || null;
        var previous = button ? button.innerHTML : '';
        if (button) {
            button.disabled = true;
            button.classList.add('is-busy');
            button.setAttribute('aria-busy', 'true');
        }

        try {
            var body;
            if (options.form) {
                body = new FormData(options.form);
            } else if (options.body instanceof FormData) {
                body = options.body;
            } else {
                body = new FormData();
                var data = options.data || {};
                Object.keys(data).forEach(function(key) {
                    var value = data[key];
                    if (value !== undefined && value !== null) body.set(key, value);
                });
            }
            if (!body.has('csrf')) {
                var token = window.clanCSRFToken();
                if (token) body.set('csrf', token);
            }

            var res = await fetch(url, {
                method: 'POST',
                credentials: 'same-origin',
                headers: {
                    'Accept': 'application/json',
                    'X-Requested-With': 'XMLHttpRequest'
                },
                body: body
            });

            var text = await res.text();
            var payload = {};
            if (text) {
                try { payload = JSON.parse(text); } catch (e) { payload = {}; }
            }

            if (!res.ok) {
                var error = new Error(payload.error || 'Something went wrong. Try again.');
                error.status = res.status;
                error.auth = res.status === 401;
                throw error;
            }
            return payload;
        } finally {
            if (button) {
                button.disabled = false;
                button.classList.remove('is-busy');
                button.removeAttribute('aria-busy');
                button.innerHTML = previous;
            }
        }
    };

    window.clanShowError = function(title, message) {
        var text = message || 'Something went wrong. Try again.';
        if (typeof window.showModal !== 'function') {
            window.alert(text);
            return;
        }
        var body = document.createElement('div');
        body.className = 'cln-modal-body';
        var msg = document.createElement('p');
        msg.className = 'cln-modal-msg';
        msg.textContent = text;
        var act = document.createElement('div');
        act.className = 'cln-modal-act';
        var dismiss = document.createElement('button');
        dismiss.type = 'button';
        dismiss.className = 'primary hpyinf hpyinl hpysm';
        var label = document.createElement('span');
        label.textContent = 'OK';
        dismiss.appendChild(label);
        dismiss.addEventListener('click', function() { window.closeModal(); });
        act.appendChild(dismiss);
        body.appendChild(msg);
        body.appendChild(act);
        window.showModal({ title: title || "That didn't work", body: body });
    };

    var clanMsgTimer = null;

    var clanMessageField = function(form, message) {
        var text = String(message || '').toLowerCase();
        var selector = null;

        if (text.indexOf('tag color') !== -1) selector = 'input[name="tag_color"]';
        else if (text.indexOf('tag') !== -1) selector = 'input[name="tag"]';
        else if (text.indexOf('description') !== -1) selector = 'textarea[name="description"]';
        else if (text.indexOf('name') !== -1) selector = 'input[name="name"]';
        else if (text.indexOf('rank') !== -1) selector = 'input[name="rank"]';
        else if (text.indexOf('title') !== -1) selector = 'input[name="title"]';
        else if (text.indexOf('image') !== -1 || text.indexOf('icon') !== -1) selector = 'input[name="icon"]';
        else if (text.indexOf('announcement') !== -1 || text.indexOf('wall') !== -1 || text.indexOf('write something') !== -1) selector = 'textarea[name="body"]';
        else if (text.indexOf('permission') !== -1) selector = '.cln-perm-grid';

        return (selector && form.querySelector(selector)) || form.querySelector('input:not([type="hidden"]), textarea');
    };

    var clanPlaceMessage = function(field, className, message) {
        var parent = field.closest('.instkrw') || field.closest('.crtfld') || field.closest('.ttpcnt') || field.parentElement;
        if (!parent) return;
        var node = parent.querySelector('.' + className);
        if (!node) {
            node = document.createElement('span');
            node.className = className;
            parent.appendChild(node);
        }
        node.textContent = message;
        requestAnimationFrame(function() { node.classList.add('visible'); });
    };

    var clanDismissMessages = function() {
        document.querySelectorAll('.frminsuc.visible').forEach(function(el) {
            el.classList.remove('visible');
        });
    };

    var clanScheduleDismiss = function() {
        if (clanMsgTimer) clearTimeout(clanMsgTimer);
        clanMsgTimer = setTimeout(function() {
            clanDismissMessages();
            clanMsgTimer = null;
        }, 2000);
    };

    window.clanClearMessages = function(form) {
        if (!form) return;
        form.querySelectorAll('.setinbox, .setareabox').forEach(function(el) {
            el.classList.remove('inerr');
        });
        form.querySelectorAll('.frminerr, .frminsuc').forEach(function(el) {
            el.remove();
        });
    };

    window.clanFormError = function(form, message) {
        if (!form) return;
        var field = clanMessageField(form, message);
        if (!field) return;
        var box = field.closest('.setinbox') || field.closest('.setareabox');
        if (box) box.classList.add('inerr');
        clanPlaceMessage(field, 'frminerr', message);
    };

    window.clanHandleError = function(err, title, form) {
        if (err && err.auth) {
            window.location.href = '/login';
            return;
        }
        var message = err && err.message ? err.message : '';
        if (form) {
            window.clanClearMessages(form);
            window.clanFormError(form, message || 'Something went wrong. Try again.');
            return;
        }
        window.clanShowError(title, message);
    };

    window.clanConfirm = function(options) {
        options = options || {};
        if (typeof window.showModal !== 'function') {
            if (window.confirm(options.message || 'Are you sure?')) options.onConfirm();
            return;
        }
        var body = document.createElement('div');
        body.className = 'cln-modal-body';
        var msg = document.createElement('p');
        msg.className = 'cln-modal-msg';
        msg.textContent = options.message || 'Are you sure?';
        var act = document.createElement('div');
        act.className = 'cln-modal-act';

        var cancel = document.createElement('button');
        cancel.type = 'button';
        cancel.className = 'primary hpyinf hpyinl hpysm';
        var cancelLabel = document.createElement('span');
        cancelLabel.textContent = options.cancelLabel || 'Cancel';
        cancel.appendChild(cancelLabel);
        cancel.addEventListener('click', function() { window.closeModal(); });

        var confirm = document.createElement('button');
        confirm.type = 'button';
        confirm.className = 'primary hpyinl hpysm ' + (options.danger ? 'hpydng' : 'hpyprim');
        var confirmLabel = document.createElement('span');
        confirmLabel.textContent = options.confirmLabel || 'Confirm';
        confirm.appendChild(confirmLabel);
        confirm.addEventListener('click', function() {
            window.closeModal();
            if (typeof options.onConfirm === 'function') options.onConfirm();
        });

        act.appendChild(cancel);
        act.appendChild(confirm);
        body.appendChild(msg);
        body.appendChild(act);
        window.showModal({ title: options.title || 'Are you sure?', subtitle: options.subtitle || '', body: body });
    };

    window.clanRefresh = function(url) {
        var target = url || (window.location.pathname + window.location.search);
        if (typeof window.navigateWithHistory === 'function') {
            window.navigateWithHistory(target);
            return;
        }
        window.location.href = target;
    };

    window.clanRefreshWithNotice = function(path, message) {
        var url = path;
        if (message) url += (url.indexOf('?') === -1 ? '?' : '&') + 'notice=' + encodeURIComponent(message);
        window.clanRefresh(url);
    };

    window.clanIsInsideHidden = function(el) {
        return !!el.closest('[hidden]');
    };

    window.clanIconError = function(file) {
        if (!file) return '';
        var typeOk = /^image\/(png|jpeg)$/i.test(file.type || '');
        var nameOk = /\.(png|jpe?g)$/i.test(file.name || '');
        if (!typeOk && !nameOk) return 'Choose a PNG or JPG image.';
        if (file.size > 8 * 1024 * 1024) return 'That image is larger than 8 MB.';
        return '';
    };

    window.clanPreviewImage = function(file, img) {
        if (!file || !img) return;
        var reader = new FileReader();
        reader.onload = function() {
            img.src = String(reader.result || '');
            img.hidden = false;
        };
        reader.readAsDataURL(file);
    };

    window.clanTidyTag = function(input) {
        if (!input) return '';
        var tag = String(input.value || '').toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 5);
        if (input.value !== tag) input.value = tag;
        return tag;
    };

    window.normalizeClanTagInput = function(input) {
        var tag = window.clanTidyTag(input);
        if (input && input.id === 'clnEditTag') {
            var preview = document.getElementById('clnEditPreviewTag');
            if (preview) {
                preview.textContent = tag ? '[' + tag + ']' : '';
                preview.hidden = !tag;
            }
        }
        return tag;
    };

    var clanTagPickers = {};

    window.initClanTagColorPicker = function(prefix) {
        var box = document.getElementById(prefix + 'TagColorPicker');
        if (!box || box.dataset.pickerInit) return null;
        if (typeof window.GlobalColorPicker !== 'function') return null;
        box.dataset.pickerInit = '1';

        var input = document.getElementById(prefix + 'TagColor');
        var picker = new window.GlobalColorPicker(box, {
            width: 170,
            initialColor: input && input.value ? input.value : '#912ADF',
            onChange: function(hex) {
                if (input) input.value = hex;
                var swatch = document.getElementById(prefix + 'TagSwatch');
                if (swatch) swatch.style.background = hex;
                if (prefix === 'clnEdit') {
                    var preview = document.getElementById('clnEditPreviewTag');
                    if (preview) preview.style.setProperty('--cln-tag-color', hex);
                }
            }
        });
        clanTagPickers[prefix] = picker;
        return picker;
    };

    window.toggleClanTagPicker = function(prefix) {
        var pop = document.getElementById(prefix + 'TagColorPop');
        if (!pop) return;
        window.initClanTagColorPicker(prefix);
        pop.classList.toggle('open');
    };

    window.closeClanTagPicker = function(prefix) {
        var pop = document.getElementById(prefix + 'TagColorPop');
        if (pop) pop.classList.remove('open');
    };

    window.toggleWornClanTag = function(clanID, btn) {
        (async function() {
            try {
                var payload = await window.clanPost('/clans/' + clanID + '/tag', { button: btn });
                window.clanRefreshWithNotice(window.location.pathname, payload.wearing ? 'Clan tag equipped.' : 'Clan tag removed.');
            } catch (err) {
                window.clanHandleError(err, "Couldn't update your clan tag");
            }
        })();
    };

    window.submitClanAnnouncement = function(event, clanID) {
        if (event) event.preventDefault();
        var form = event && event.target && event.target.closest ? event.target.closest('form') : null;
        if (!form) return false;

        window.clanClearMessages(form);

        var body = form.querySelector('[name="body"]');
        if (body && !body.value.trim()) {
            window.clanFormError(form, 'Write something first.');
            body.focus();
            return false;
        }

        var btn = form.querySelector('button[type="submit"]');
        (async function() {
            try {
                await window.clanPost('/clans/' + clanID + '/announcements', { form: form, button: btn });
                window.clanRefreshWithNotice(window.location.pathname, 'Announcement posted.');
            } catch (err) {
                window.clanHandleError(err, "Couldn't post that announcement", form);
            }
        })();
        return false;
    };

    window.deleteClanAnnouncement = function(clanID, announcementID, btn) {
        window.clanConfirm({
            title: 'Delete this announcement?',
            message: 'It will disappear from the clan home page for everyone.',
            confirmLabel: 'Delete',
            danger: true,
            onConfirm: function() {
                (async function() {
                    try {
                        await window.clanPost('/clans/' + clanID + '/announcements/' + announcementID + '/delete', { button: btn });
                        window.clanRefreshWithNotice(window.location.pathname, 'Announcement deleted.');
                    } catch (err) {
                        window.clanHandleError(err, "Couldn't delete that announcement");
                    }
                })();
            }
        });
    };

    var clanInitMessages = function() {
        if (document.querySelector('.frminsuc.visible')) clanScheduleDismiss();
    };

    clanInitMessages();
    document.addEventListener('htmx:afterSettle', clanInitMessages);
})();
