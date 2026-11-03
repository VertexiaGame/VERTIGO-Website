(function() {
    window.clanCreateCat = 'social';

    const categoryLabels = {
        building: 'Building',
        social: 'Social',
        grinding: 'Grinding',
        roleplay: 'Roleplay',
        games: 'Games',
        trading: 'Trading',
        creative: 'Creative',
        music: 'Music',
        ugc: 'UGC'
    };

    const categoryIcons = {
        building: 'fa-cubes',
        social: 'fa-comments',
        grinding: 'fa-bolt',
        roleplay: 'fa-masks-theater',
        games: 'fa-gamepad',
        trading: 'fa-handshake',
        creative: 'fa-palette',
        music: 'fa-music',
        ugc: 'fa-shirt'
    };

    const updateCategoryPreview = function() {
        const text = document.getElementById('crtClanPreviewCat');
        if (text) text.textContent = categoryLabels[window.clanCreateCat] || 'Social';
        const icon = document.getElementById('crtClanPreviewCatIcon');
        if (icon) icon.className = 'fa-solid ' + (categoryIcons[window.clanCreateCat] || 'fa-layer-group');
    };

    const updatePrivacyPreview = function() {
        const input = document.getElementById('crtClanPrivacyInput');
        const approval = input ? input.value === 'approval' : false;

        const lock = document.getElementById('crtClanPreviewLock');
        if (lock) lock.hidden = !approval;

        const note = document.getElementById('crtClanPrivacyNote');
        if (note) note.classList.toggle('is-visible', approval);

        const icon = document.getElementById('crtClanPreviewPrivacyIcon');
        if (icon) icon.className = 'fa-solid ' + (approval ? 'fa-lock' : 'fa-earth-americas');
        const text = document.getElementById('crtClanPreviewPrivacyText');
        if (text) text.textContent = approval ? 'Approval only' : 'Public';
    };

    window.updateClanCreatePreview = function() {
        const nameInput = document.getElementById('crtClanName');
        const nameEl = document.getElementById('crtClanPreviewName');
        if (!nameInput) return;

        const name = nameInput.value.trim();
        if (nameEl) nameEl.textContent = name || 'Untitled Clan';
    };

    window.switchClanCreateCategory = function(cat, event) {
        const bar = document.getElementById('crtClanCatBar');
        const pill = event && event.currentTarget
            ? event.currentTarget
            : (bar ? bar.querySelector('.gam-pill[data-cat="' + cat + '"]') : null);

        if (pill && pill.classList.contains('active') && event) return;

        window.clanCreateCat = cat || 'social';
        if (bar) {
            bar.querySelectorAll('.gam-pill').forEach(function(item) {
                item.classList.toggle('active', item.dataset.cat === window.clanCreateCat);
            });
        }

        const input = document.getElementById('crtClanCatInput');
        if (input) input.value = window.clanCreateCat;
        updateCategoryPreview();
    };

    window.handleClanIcon = function(event) {
        const input = event && event.target ? event.target : document.getElementById('crtClanIcon');
        const file = input && input.files ? input.files[0] : null;
        const msgEl = document.getElementById('crtClanIconMsg');
        const nameEl = document.getElementById('crtClanIconName');

        const error = window.clanIconError(file);
        if (error) {
            input.value = '';
            window.clanShowError("That icon won't work", error);
            return;
        }

        if (nameEl) nameEl.textContent = file ? file.name : '';
        if (msgEl) msgEl.textContent = file ? 'Choose a different image' : 'Choose a PNG or JPG';

        const img = document.getElementById('crtClanPreviewIcon');
        if (file) {
            window.clanPreviewImage(file, img);
        } else if (img) {
            img.hidden = true;
            img.removeAttribute('src');
        }
    };

    window.handleClanBanner = function(event) {
        const input = event && event.target ? event.target : document.getElementById('crtClanBanner');
        const file = input && input.files ? input.files[0] : null;
        const msgEl = document.getElementById('crtClanBannerMsg');
        const nameEl = document.getElementById('crtClanBannerName');

        const error = window.clanIconError(file);
        if (error) {
            input.value = '';
            window.clanShowError("That banner won't work", error);
            return;
        }

        if (nameEl) nameEl.textContent = file ? file.name : '';
        if (msgEl) msgEl.textContent = file ? 'Choose a different image' : 'Choose a PNG or JPG';

        const wrap = document.getElementById('crtClanPreviewBanner');
        const img = document.getElementById('crtClanPreviewBannerImg');
        if (file) {
            window.clanPreviewImage(file, img);
            if (wrap) wrap.hidden = false;
        } else {
            if (img) {
                img.hidden = true;
                img.removeAttribute('src');
            }
            if (wrap) wrap.hidden = true;
        }
    };

    window.submitClanCreate = function(event) {
        if (event) {
            event.preventDefault();
            event.stopPropagation();
        }

        const form = document.getElementById('crtClanForm');
        const nameInput = document.getElementById('crtClanName');
        if (!form || !nameInput) return false;

        const name = nameInput.value.trim();
        const nameBox = nameInput.closest('.setinbox');
        if (name.length < 3 || name.length > 40) {
            if (nameBox) nameBox.classList.add('inerr');
            nameInput.focus();
            setTimeout(function() { if (nameBox) nameBox.classList.remove('inerr'); }, 1200);
            window.clanShowError('Check the clan name', 'Clan names need to be between 3 and 40 characters.');
            return false;
        }

        const tagInput = document.getElementById('crtClanTag');
        const tag = window.clanTidyTag(tagInput);
        if (!/^[A-Z0-9]{2,5}$/.test(tag)) {
            const tagBox = tagInput ? tagInput.closest('.setinbox') : null;
            if (tagBox) tagBox.classList.add('inerr');
            if (tagInput) tagInput.focus();
            setTimeout(function() { if (tagBox) tagBox.classList.remove('inerr'); }, 1200);
            window.clanShowError('Check the clan tag', 'Clan tags need to be 2 to 5 letters or numbers (A-Z, 0-9).');
            return false;
        }

        const iconInput = document.getElementById('crtClanIcon');
        const file = iconInput && iconInput.files ? iconInput.files[0] : null;
        const iconError = window.clanIconError(file);
        if (iconError) {
            window.clanShowError("That icon won't work", iconError);
            return false;
        }

        const bannerInput = document.getElementById('crtClanBanner');
        const bannerFile = bannerInput && bannerInput.files ? bannerInput.files[0] : null;
        const bannerError = window.clanIconError(bannerFile);
        if (bannerError) {
            window.clanShowError("That banner won't work", bannerError);
            return false;
        }

        const btn = document.getElementById('crtClanSubmitBtn');
        (async function() {
            try {
                const payload = await window.clanPost('/create/clan', { form: form, button: btn });
                if (typeof window.navigateWithHistory === 'function') {
                    window.navigateWithHistory(payload.url || '/clans');
                } else {
                    window.location.href = payload.url || '/clans';
                }
            } catch (err) {
                window.clanHandleError(err, "Couldn't create your clan");
            }
        })();
        return false;
    };

    window.initClanCreate = function() {
        if (!document.getElementById('crtClanName')) return;

        const activeCat = document.querySelector('#crtClanCatBar .gam-pill.active');
        window.clanCreateCat = activeCat && activeCat.dataset.cat ? activeCat.dataset.cat : 'social';
        const catInput = document.getElementById('crtClanCatInput');
        if (catInput) catInput.value = window.clanCreateCat;

        const nameInput = document.getElementById('crtClanName');
        if (nameInput && nameInput.dataset.clnBound !== '1') {
            nameInput.dataset.clnBound = '1';
            nameInput.addEventListener('input', function() {
                const box = nameInput.closest('.setinbox');
                if (box) box.classList.remove('inerr');
            });
        }

        const privacy = document.getElementById('crtClanPrivacyInput');
        if (privacy) privacy.onchange = updatePrivacyPreview;

        const iconInput = document.getElementById('crtClanIcon');
        const bannerInput = document.getElementById('crtClanBanner');

        updateCategoryPreview();
        updatePrivacyPreview();
        window.updateClanCreatePreview();

        if (iconInput && iconInput.files && iconInput.files.length) {
            window.handleClanIcon({ target: iconInput });
        }
        if (bannerInput && bannerInput.files && bannerInput.files.length) {
            window.handleClanBanner({ target: bannerInput });
        }
    };

    window.initClanCreate();
    document.addEventListener('htmx:afterSettle', window.initClanCreate);
})();
