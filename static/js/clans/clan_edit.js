(function() {
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

    const firstLetter = function(value) {
        const chars = Array.from(String(value || '').trim());
        if (!chars.length) return 'C';
        const ch = chars[0];
        return /[a-z]/i.test(ch) ? ch.toUpperCase() : 'C';
    };

    window.updateClanEditPreviewName = function(value) {
        const input = document.getElementById('clnEditName');
        const name = value == null ? (input ? input.value : '') : value;
        const nameEl = document.getElementById('clnEditPreviewName');
        const letterEl = document.getElementById('clnEditPreviewLetter');
        if (nameEl) nameEl.textContent = name.trim() || 'Untitled Clan';
        if (letterEl) letterEl.textContent = firstLetter(name);
    };

    const updatePrivacyPreview = function() {
        const input = document.getElementById('clnEditPrivacy');
        const approval = input ? input.value === 'approval' : false;

        const lock = document.getElementById('clnEditPreviewLock');
        if (lock) lock.hidden = !approval;

        const note = document.getElementById('clnEditPrivacyNote');
        if (note) note.classList.toggle('is-visible', approval);

        const item = document.getElementById('clnPrevPrivacy');
        if (item) {
            const icon = item.querySelector('i');
            if (icon) icon.className = 'fa-solid ' + (approval ? 'fa-lock' : 'fa-earth-americas');
            const text = document.getElementById('clnPrevPrivacyText');
            if (text) text.textContent = approval ? 'Approval only' : 'Public';
        }
    };

    const updateCategoryPreview = function() {
        const input = document.getElementById('clnEditCat');
        const key = input ? input.value : 'social';
        const text = document.getElementById('clnPrevCategoryText');
        if (text) text.textContent = categoryLabels[key] || 'Social';
        const item = document.getElementById('clnPrevCategory');
        if (item) {
            const icon = item.querySelector('i');
            if (icon) icon.className = 'fa-solid ' + (categoryIcons[key] || 'fa-layer-group');
        }
    };

    window.switchClanEditCategory = function(category, event) {
        if (event) {
            event.preventDefault();
            event.stopPropagation();
        }
        const bar = document.getElementById('clnEditCatBar');
        if (bar) {
            bar.querySelectorAll('.gam-pill').forEach(function(pill) {
                pill.classList.toggle('active', pill.dataset.cat === category);
            });
        }
        const input = document.getElementById('clnEditCat');
        if (input) input.value = category;
        updateCategoryPreview();
    };

    window.handleClanEditIcon = function(event) {
        const input = event && event.target ? event.target : document.getElementById('clnEditIcon');
        const file = input && input.files ? input.files[0] : null;
        const img = document.getElementById('clnEditPreviewIcon');
        const nameEl = document.getElementById('clnEditIconName');
        const msgEl = document.getElementById('clnEditIconMsg');

        const error = window.clanIconError(file);
        if (error) {
            input.value = '';
            const form = document.getElementById('clnProfileForm');
            if (form) {
                window.clanClearMessages(form);
                window.clanFormError(form, error);
            }
            return;
        }

        const clear = document.querySelector('#clnProfileForm input[name="clear_icon"]');
        if (clear && clear.checked) {
            clear.checked = false;
            window.updateClanEditPreviewIconClear(false);
        }

        if (nameEl) nameEl.textContent = file ? file.name : '';
        if (msgEl) msgEl.textContent = file ? 'Ready to upload' : 'Choose a new icon';
        if (file) window.clanPreviewImage(file, img);
    };

    window.updateClanEditPreviewIconClear = function(checked) {
        const img = document.getElementById('clnEditPreviewIcon');
        if (!img) return;
        if (checked) {
            img.hidden = true;
            img.removeAttribute('src');
            return;
        }
        const original = img.dataset.originalSrc || '';
        if (original) {
            img.src = original;
            img.hidden = false;
        } else {
            img.hidden = true;
        }
    };

    window.submitClanProfile = function(event, clanID) {
        if (event) event.preventDefault();
        const form = document.getElementById('clnProfileForm');
        if (!form) return false;

        window.clanClearMessages(form);

        const nameInput = document.getElementById('clnEditName');
        const name = nameInput ? nameInput.value.trim() : '';
        if (name.length < 3 || name.length > 40) {
            window.clanFormError(form, 'Clan names need to be between 3 and 40 characters.');
            if (nameInput) nameInput.focus();
            return false;
        }

        const tagInput = document.getElementById('clnEditTag');
        const tag = window.clanTidyTag(tagInput);
        if (!/^[A-Z0-9]{2,5}$/.test(tag)) {
            window.clanFormError(form, 'Clan tags need to be 2 to 5 letters or numbers (A-Z, 0-9).');
            if (tagInput) tagInput.focus();
            return false;
        }

        const iconInput = document.getElementById('clnEditIcon');
        const file = iconInput && iconInput.files ? iconInput.files[0] : null;
        const iconError = window.clanIconError(file);
        if (iconError) {
            window.clanFormError(form, iconError);
            return false;
        }

        const btn = document.getElementById('clnProfileSubmit');
        (async function() {
            try {
                await window.clanPost('/create/clans/' + clanID, { form: form, button: btn });
                window.clanRefresh('/create/clans/' + clanID + '/edit');
            } catch (err) {
                window.clanHandleError(err, "Couldn't save your changes", form);
            }
        })();
        return false;
    };

    const roleFormEl = function() {
        return document.getElementById('clnRoleForm');
    };

    const roleRows = function() {
        return Array.prototype.slice.call(document.querySelectorAll('#clnEditRoles .cln-rolecard'));
    };

    const normalizeRoleColor = function(value) {
        const color = String(value || '').trim();
        return /^#[0-9a-f]{6}$/i.test(color) ? color.toLowerCase() : '#912adf';
    };

    const roleMemberLabel = function(value) {
        const count = parseInt(value, 10) || 0;
        return count + (count === 1 ? ' member' : ' members');
    };

    const roleMetaLabel = function(rank, members) {
        const label = roleMemberLabel(members);
        return rank ? 'Rank ' + rank + ', ' + label : label;
    };

    const showRoleView = function(view) {
        const editor = document.getElementById('clnRoleEditor');
        const owner = document.getElementById('clnRoleOwner');
        const form = roleFormEl();
        const empty = document.getElementById('clnRoleEmpty');

        if (owner) owner.hidden = view !== 'owner';
        if (form) form.hidden = view !== 'form';
        if (empty) empty.hidden = view !== 'none';
        if (!editor) return;

        editor.hidden = view === 'none';
        if (view === 'none') return;

        editor.classList.remove('is-swapping');
        void editor.offsetWidth;
        editor.classList.add('is-swapping');
    };

    const scrollToRoleEditor = function() {
        if (!window.matchMedia || !window.matchMedia('(max-width: 980px)').matches) return;
        const editor = document.getElementById('clnRoleEditor');
        if (editor) editor.scrollIntoView({ behavior: 'smooth', block: 'center' });
    };

    const clearRoleSelection = function() {
        roleRows().forEach(function(row) {
            row.classList.remove('is-selected');
            const dot = row.querySelector('.cln-rolecard-dot');
            if (dot) dot.style.backgroundColor = row.dataset.roleColor || '';
        });
    };

    const syncRoleColor = function(rawColor) {
        const color = normalizeRoleColor(rawColor);
        const input = document.getElementById('clnRoleColor');
        if (input) input.value = color;
        const swatch = document.getElementById('clnRoleSwatch');
        if (swatch) swatch.style.background = color;
        const val = document.getElementById('clnRoleColorVal');
        if (val) val.textContent = color.toUpperCase();

        const avatar = document.getElementById('clnRoleAvatar');
        if (avatar) avatar.style.setProperty('--cln-role-color', color);

        const selected = document.querySelector('#clnEditRoles .cln-rolecard.is-selected');
        if (selected) {
            const dot = selected.querySelector('.cln-rolecard-dot');
            if (dot) dot.style.backgroundColor = color;
        }
    };

    const applyRoleColor = function(rawColor) {
        const color = normalizeRoleColor(rawColor);
        syncRoleColor(color);
        if (window.clanRoleColorPicker) window.clanRoleColorPicker.setColor(color);
    };

    const syncRolePermissions = function() {
        const form = roleFormEl();
        if (!form) return;
        const boxes = form.querySelectorAll('input[name="permissions"]');
        let active = 0;
        boxes.forEach(function(box) {
            if (box.checked) active++;
            const tile = box.closest('.cln-perm');
            if (tile) tile.classList.toggle('is-on', box.checked);
        });
        const count = document.getElementById('clnRolePermCount');
        if (count) {
            count.textContent = active + ' of ' + boxes.length + ' active';
            count.classList.toggle('has-perms', active > 0);
        }
    };

    const resetRoleForm = function() {
        const form = roleFormEl();
        if (!form) return;
        form.reset();
        window.closeClanRolePicker();

        const idEl = document.getElementById('clnRoleId');
        const titleEl = document.getElementById('clnRoleFormTitle');
        const subEl = document.getElementById('clnRoleFormSub');
        const rankEl = document.getElementById('clnRoleRank');
        const colorEl = document.getElementById('clnRoleColor');
        const delBtn = document.getElementById('clnRoleDelete');
        const note = document.getElementById('clnRoleDefaultNote');

        if (idEl) idEl.value = '';
        if (titleEl) titleEl.textContent = 'New role';
        if (subEl) subEl.textContent = 'Give the role a name, colour and permissions';
        if (rankEl) rankEl.value = '50';
        if (colorEl) colorEl.value = '#912ADF';
        if (delBtn) delBtn.hidden = true;
        if (note) note.hidden = true;

        applyRoleColor('#912ADF');
        syncRolePermissions();
    };

    window.selectClanRole = function(el) {
        if (!el) return;
        const card = el.closest ? (el.closest('.cln-rolecard') || el) : el;
        window.closeClanRolePicker();

        roleRows().forEach(function(row) {
            row.classList.toggle('is-selected', row === card);
        });

        if (card.dataset.roleOwner === '1') {
            const title = document.getElementById('clnOwnerTitle');
            if (title) title.textContent = card.dataset.roleName || 'Owner';
            const meta = document.getElementById('clnOwnerMeta');
            if (meta) meta.textContent = roleMetaLabel(card.dataset.roleRank, card.dataset.roleMembers);
            const ownerAvatar = document.getElementById('clnOwnerAvatar');
            if (ownerAvatar) ownerAvatar.style.setProperty('--cln-role-color', normalizeRoleColor(card.dataset.roleColor));
            showRoleView('owner');
            scrollToRoleEditor();
            return;
        }

        const form = roleFormEl();
        if (!form) return;
        form.reset();

        const idEl = document.getElementById('clnRoleId');
        const nameEl = document.getElementById('clnRoleName');
        const rankEl = document.getElementById('clnRoleRank');
        const colorEl = document.getElementById('clnRoleColor');
        const titleEl = document.getElementById('clnRoleFormTitle');
        const subEl = document.getElementById('clnRoleFormSub');
        const delBtn = document.getElementById('clnRoleDelete');
        const note = document.getElementById('clnRoleDefaultNote');
        const color = normalizeRoleColor(card.dataset.roleColor);
        const name = card.dataset.roleName || '';

        if (idEl) idEl.value = card.dataset.roleId || '';
        if (nameEl) nameEl.value = name;
        if (rankEl) rankEl.value = card.dataset.roleRank || '50';
        if (colorEl) colorEl.value = color;
        if (titleEl) titleEl.textContent = name || 'Edit role';
        if (subEl) subEl.textContent = roleMetaLabel(card.dataset.roleRank || '50', card.dataset.roleMembers);
        if (delBtn) delBtn.hidden = card.dataset.roleDefault === '1';
        if (note) note.hidden = card.dataset.roleDefault !== '1';

        const perms = (card.dataset.rolePerms || '').split(',').filter(Boolean);
        form.querySelectorAll('input[name="permissions"]').forEach(function(box) {
            box.checked = perms.indexOf(box.value) !== -1;
        });

        applyRoleColor(color);
        syncRolePermissions();
        showRoleView('form');
        scrollToRoleEditor();
    };

    window.newClanRole = function() {
        clearRoleSelection();
        resetRoleForm();
        showRoleView('form');
        scrollToRoleEditor();

        const nameEl = document.getElementById('clnRoleName');
        if (nameEl) setTimeout(function() { nameEl.focus({ preventScroll: true }); }, 200);
    };

    window.closeClanRoleEditor = function() {
        clearRoleSelection();
        resetRoleForm();
        showRoleView('none');
    };

    window.initClanRoleColorPicker = function() {
        const box = document.getElementById('clnRoleColorPicker');
        if (!box || box.dataset.pickerInit) return;
        if (typeof window.GlobalColorPicker !== 'function') return;
        box.dataset.pickerInit = '1';

        const input = document.getElementById('clnRoleColor');
        window.clanRoleColorPicker = new window.GlobalColorPicker(box, {
            width: 170,
            initialColor: normalizeRoleColor(input ? input.value : '#912ADF'),
            onChange: function(hex) {
                syncRoleColor(hex);
            }
        });
    };

    window.toggleClanRolePicker = function() {
        const pop = document.getElementById('clnRoleColorPop');
        if (!pop) return;
        window.initClanRoleColorPicker();
        const box = document.getElementById('clnRoleColorPicker');
        if (!box || !box.dataset.pickerInit) return;
        pop.classList.toggle('open');
    };

    window.closeClanRolePicker = function() {
        const pop = document.getElementById('clnRoleColorPop');
        if (pop) pop.classList.remove('open');
    };

    window.submitClanRole = function(event, clanID) {
        if (event) event.preventDefault();
        const form = roleFormEl();
        if (!form) return false;

        window.clanClearMessages(form);

        const roleID = (document.getElementById('clnRoleId') || {}).value || '';
        const nameEl = document.getElementById('clnRoleName');
        const name = nameEl ? nameEl.value.trim() : '';
        if (!name) {
            window.clanFormError(form, 'Role name is required.');
            if (nameEl) nameEl.focus();
            return false;
        }

        const rankEl = document.getElementById('clnRoleRank');
        const rank = parseInt(rankEl ? rankEl.value : '50', 10);
        if (isNaN(rank) || rank < 1 || rank > 99) {
            window.clanFormError(form, 'Role ranks need to be between 1 and 99.');
            if (rankEl) rankEl.focus();
            return false;
        }

        const btn = document.getElementById('clnRoleSubmit');
        const url = '/clans/' + clanID + '/roles' + (roleID ? '/' + roleID : '');
        (async function() {
            try {
                await window.clanPost(url, { form: form, button: btn });
                window.clanRefresh('/create/clans/' + clanID + '/edit?tab=roles');
            } catch (err) {
                window.clanHandleError(err, "Couldn't save that role", form);
            }
        })();
        return false;
    };

    window.deleteClanRole = function(clanID, btn) {
        const roleID = (document.getElementById('clnRoleId') || {}).value || '';
        if (!roleID) return;
        const nameEl = document.getElementById('clnRoleName');
        const name = nameEl ? nameEl.value.trim() : '';

        window.clanConfirm({
            title: 'Delete ' + (name || 'this role') + '?',
            message: 'Members with this role will move to the default role.',
            confirmLabel: 'Delete',
            danger: true,
            onConfirm: function() {
                (async function() {
                    try {
                        await window.clanPost('/clans/' + clanID + '/roles/' + roleID + '/delete', { button: btn });
                        window.clanRefresh('/create/clans/' + clanID + '/edit?tab=roles');
                    } catch (err) {
                        window.clanHandleError(err, "Couldn't delete that role");
                    }
                })();
            }
        });
    };

    window.approveClanRequest = function(clanID, requestID, btn) {
        const row = btn ? btn.closest('.cln-req-row') : null;
        const select = row ? row.querySelector('[data-request-role]') : null;
        const roleID = select ? select.value : '';

        (async function() {
            try {
                await window.clanPost('/clans/' + clanID + '/requests/' + requestID + '/approve', {
                    data: { role_id: roleID },
                    button: btn
                });
                window.clanRefresh('/create/clans/' + clanID + '/edit');
            } catch (err) {
                window.clanHandleError(err, "Couldn't approve that request");
            }
        })();
    };

    window.denyClanRequest = function(clanID, requestID, btn) {
        window.clanConfirm({
            title: 'Deny this request?',
            message: 'The request will be removed from the list.',
            confirmLabel: 'Deny',
            danger: true,
            onConfirm: function() {
                (async function() {
                    try {
                        await window.clanPost('/clans/' + clanID + '/requests/' + requestID + '/deny', { button: btn });
                        window.clanRefresh('/create/clans/' + clanID + '/edit');
                    } catch (err) {
                        window.clanHandleError(err, "Couldn't deny that request");
                    }
                })();
            }
        });
    };

    window.saveClanMemberRole = function(clanID, userID, btn) {
        const row = btn ? btn.closest('.cln-mgmt-row') : null;
        const select = row ? row.querySelector('[data-member-role]') : null;
        const roleID = select ? select.value : '';
        if (!roleID) return;

        (async function() {
            try {
                await window.clanPost('/clans/' + clanID + '/members/' + userID + '/role', {
                    data: { role_id: roleID },
                    button: btn
                });
                window.clanRefresh('/create/clans/' + clanID + '/edit');
            } catch (err) {
                window.clanHandleError(err, "Couldn't update that member");
            }
        })();
    };

    window.kickClanMember = function(clanID, userID, btn) {
        window.clanConfirm({
            title: 'Kick this member?',
            message: 'They will be removed from the clan and can request to join again.',
            confirmLabel: 'Kick',
            danger: true,
            onConfirm: function() {
                (async function() {
                    try {
                        await window.clanPost('/clans/' + clanID + '/members/' + userID + '/kick', { button: btn });
                        window.clanRefresh('/create/clans/' + clanID + '/edit');
                    } catch (err) {
                        window.clanHandleError(err, "Couldn't kick that member");
                    }
                })();
            }
        });
    };

    const filterEditMembers = function() {
        const search = document.getElementById('clnEditMemberSearch');
        if (!search) return;
        const q = search.value.trim().toLowerCase();
        let shown = 0;

        document.querySelectorAll('#clnEditMembers .cln-mgmt-row').forEach(function(row) {
            const name = (row.dataset.name || '').toLowerCase();
            const match = !q || name.indexOf(q) !== -1;
            row.hidden = !match;
            if (match) shown++;
        });

        const empty = document.getElementById('clnEditMemberEmpty');
        if (empty) empty.hidden = shown !== 0;
    };

    window.updateClanEditTabIndicator = function() {
        const page = document.getElementById('clnEditPage');
        if (!page) return;
        const indicator = page.querySelector('.tabind');
        const activeTab = page.querySelector('.tabbtn.active');
        if (!indicator || !activeTab) return;
        indicator.style.left = activeTab.offsetLeft + 'px';
        indicator.style.width = activeTab.offsetWidth + 'px';
        indicator.style.height = activeTab.offsetHeight + 'px';
        indicator.style.top = activeTab.offsetTop + 'px';
    };

    window.switchClanEditTab = function(tabName) {
        const page = document.getElementById('clnEditPage');
        if (!page) return;

        const targetTab = page.querySelector('.tabbtn[data-cln-edit-tab="' + tabName + '"]');
        const targetContent = document.getElementById('tab-' + tabName);
        if (!targetTab || !targetContent || targetTab.classList.contains('active')) return;

        const currentContent = page.querySelector('.settabcntnt.active');

        page.querySelectorAll('.tabbtn').forEach(function(tab) {
            tab.classList.remove('active');
        });
        targetTab.classList.add('active');
        window.updateClanEditTabIndicator();

        const viewport = page.querySelector('.settabvwp');
        if (!viewport) return;

        viewport.style.height = (currentContent ? currentContent.offsetHeight : 0) + 'px';
        viewport.offsetHeight;
        if (currentContent) currentContent.classList.remove('active');
        targetContent.classList.add('active');
        viewport.style.height = targetContent.offsetHeight + 'px';

        if (window.clanEditHeightTimeout) clearTimeout(window.clanEditHeightTimeout);
        window.clanEditHeightTimeout = setTimeout(function() {
            viewport.style.height = '';
        }, 350);
    };

    const restoreClanEditTab = function() {
        const page = document.getElementById('clnEditPage');
        if (!page) return;
        const params = new URLSearchParams(window.location.search);
        const wanted = params.get('tab');
        if (!wanted || !/^[a-z-]+$/.test(wanted)) return;
        if (!page.querySelector('.tabbtn[data-cln-edit-tab="' + wanted + '"]')) return;

        params.delete('tab');
        const query = params.toString();
        window.history.replaceState({}, '', window.location.pathname + (query ? '?' + query : '') + window.location.hash);

        setTimeout(function() {
            window.switchClanEditTab(wanted);
        }, 60);
    };

    const anchorClanEditMessage = function() {
        const page = document.getElementById('clnEditPage');
        if (!page) return;
        const banner = page.querySelector('.frmmain > .frminsuc, .frmmain > .frminerr');
        if (!banner) return;

        const wanted = new URLSearchParams(window.location.search).get('tab');
        const target = (wanted && page.querySelector('#tab-' + wanted)) || page.querySelector('.settabcntnt.active');
        const form = target ? target.querySelector('form') : null;
        if (!form || form.hidden) return;

        const button = form.querySelector('button[type="submit"]');
        if (!button) return;

        const anchor = button.closest('.cln-formact') || button;
        if (anchor.parentElement) anchor.parentElement.insertBefore(banner, anchor.nextSibling);
    };

    const initClanEdit = function() {
        const page = document.getElementById('clnEditPage');
        if (!page) return;

        const privacy = document.getElementById('clnEditPrivacy');
        if (privacy) privacy.onchange = updatePrivacyPreview;

        const category = document.getElementById('clnEditCat');
        if (category) category.onchange = updateCategoryPreview;

        const roleForm = roleFormEl();
        if (roleForm && roleForm.dataset.clnBound !== '1') {
            roleForm.dataset.clnBound = '1';
            roleForm.addEventListener('change', function(e) {
                if (e.target.name === 'permissions') syncRolePermissions();
            });
        }

        const icon = document.getElementById('clnEditPreviewIcon');
        if (icon && !icon.dataset.originalSrc) {
            icon.dataset.originalSrc = icon.getAttribute('src') || '';
        }

        const memberSearch = document.getElementById('clnEditMemberSearch');
        if (memberSearch && memberSearch.dataset.clnBound !== '1') {
            memberSearch.dataset.clnBound = '1';
            memberSearch.addEventListener('input', filterEditMembers);
        }

        setTimeout(window.updateClanEditTabIndicator, 40);
        setTimeout(window.updateClanEditTabIndicator, 180);

        anchorClanEditMessage();
        updatePrivacyPreview();
        updateCategoryPreview();
        restoreClanEditTab();
    };

    initClanEdit();
    document.addEventListener('htmx:afterSettle', initClanEdit);

    if (!window.__clnEditResizeBound) {
        window.__clnEditResizeBound = true;
        window.addEventListener('resize', function() {
            if (document.getElementById('clnEditPage')) window.updateClanEditTabIndicator();
        });
    }
})();
