(function() {
    let currentClanCategory = 'all';
    let clanSearchTimer = null;

    const catNames = {
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

    window.setClanCategory = function(cat) {
        currentClanCategory = cat || 'all';
        document.querySelectorAll('#clnCatBar .gam-pill').forEach(function(p) {
            p.classList.toggle('active', p.dataset.cat === currentClanCategory);
        });
    };

    window.isClanBrowsing = function() {
        const barSearch = document.getElementById('clanBarSearch');
        const q = barSearch ? barSearch.value.trim() : '';
        return q !== '' || currentClanCategory !== 'all';
    };

    window.showClanResults = function() {
        const shelves = document.getElementById('clnShelves');
        const search = document.getElementById('clnSearch');
        if (shelves) shelves.style.display = 'none';
        if (search) {
            search.style.display = '';
            window.updateClanSearchTitle();
            window.applyClanFilters();
        }
    };

    window.showClanShelves = function() {
        const shelves = document.getElementById('clnShelves');
        const search = document.getElementById('clnSearch');
        if (shelves) shelves.style.display = '';
        if (search) search.style.display = 'none';
    };

    window.refreshClanView = function() {
        if (window.isClanBrowsing()) {
            window.showClanResults();
        } else {
            window.showClanShelves();
        }
    };

    window.updateClanSearchTitle = function() {
        const titleEl = document.getElementById('clanSearchTitle');
        if (!titleEl) return;
        const barSearch = document.getElementById('clanBarSearch');
        const q = barSearch ? barSearch.value.trim() : '';
        if (q) {
            titleEl.textContent = 'Search Results';
            return;
        }
        if (currentClanCategory && currentClanCategory !== 'all') {
            titleEl.textContent = (catNames[currentClanCategory] || currentClanCategory) + ' Clans';
        } else {
            titleEl.textContent = 'Search Results';
        }
    };

    window.switchClanCategory = function(cat, event) {
        const clickedPill = event ? event.currentTarget : null;
        if (clickedPill && clickedPill.classList.contains('active')) return;
        window.setClanCategory(cat);
        window.refreshClanView();
    };

    window.toggleClanShelf = function(btn) {
        if (!btn) return;
        const shelf = btn.closest('.gam-shelf');
        const row = shelf ? shelf.querySelector('.gam-row') : null;
        if (!row || row.dataset.shelfAnim === 'true') return;

        const cards = Array.prototype.slice.call(row.querySelectorAll('.cln-card'));
        const firstRects = cards.map(function(card) {
            return card.getBoundingClientRect();
        });

        row.dataset.shelfAnim = 'true';
        row.style.overflow = 'hidden';
        row.style.height = row.offsetHeight + 'px';
        void row.offsetHeight;

        row.classList.toggle('expanded');
        btn.textContent = row.classList.contains('expanded') ? '‹ Show Less' : 'See All ›';
        void row.offsetHeight;

        const targetHeight = row.scrollHeight;

        cards.forEach(function(card, i) {
            const lastRect = card.getBoundingClientRect();
            const dx = firstRects[i].left - lastRect.left;
            const dy = firstRects[i].top - lastRect.top;
            if (dx || dy) {
                card.style.transition = 'transform 0s';
                card.style.transform = 'translate(' + dx + 'px,' + dy + 'px)';
            }
        });
        void row.offsetHeight;

        requestAnimationFrame(function() {
            row.style.height = targetHeight + 'px';
            cards.forEach(function(card) {
                card.style.transition = 'transform 0.35s var(--ease-smooth)';
                card.style.transform = '';
            });
        });

        setTimeout(function() {
            cards.forEach(function(card) {
                card.style.transition = '';
                card.style.transform = '';
            });
            row.style.overflow = '';
            row.style.height = '';
            row.dataset.shelfAnim = 'false';
        }, 380);
    };

    window.applyClanFilters = function() {
        const grid = document.getElementById('clanGrid');
        if (!grid) return;

        const allCards = Array.prototype.slice.call(grid.querySelectorAll('.cln-card'));
        const barSearch = document.getElementById('clanBarSearch');
        const q = (barSearch ? barSearch.value : '').trim().toLowerCase();
        const sortEl = document.getElementById('clanSortSelect');
        const sort = sortEl ? sortEl.value : 'members-desc';
        const cat = currentClanCategory;

        const visible = [];
        allCards.forEach(function(card) {
            let show = true;
            if (cat !== 'all') {
                const cardCats = (card.dataset.cats || '').split(',');
                if (cardCats.indexOf(cat) === -1) show = false;
            }
            if (show && q) {
                const name = card.querySelector('.cln-nm');
                const haystack = (name ? name.textContent : card.textContent).toLowerCase();
                if (haystack.indexOf(q) === -1) show = false;
            }
            card.hidden = !show;
            if (show) visible.push(card);
        });

        visible.sort(function(a, b) {
            let av, bv;
            if (sort === 'members-asc') {
                av = parseInt(a.dataset.members || '0', 10);
                bv = parseInt(b.dataset.members || '0', 10);
                return av - bv;
            }
            if (sort === 'newest') {
                av = parseInt(a.dataset.days || '0', 10);
                bv = parseInt(b.dataset.days || '0', 10);
                return av - bv;
            }
            if (sort === 'oldest') {
                av = parseInt(a.dataset.days || '0', 10);
                bv = parseInt(b.dataset.days || '0', 10);
                return bv - av;
            }
            if (sort === 'name-asc') {
                av = (a.querySelector('.cln-nm') || { textContent: '' }).textContent.toLowerCase();
                bv = (b.querySelector('.cln-nm') || { textContent: '' }).textContent.toLowerCase();
                return av.localeCompare(bv);
            }
            av = parseInt(a.dataset.members || '0', 10);
            bv = parseInt(b.dataset.members || '0', 10);
            return bv - av;
        });
        visible.forEach(function(card) { grid.appendChild(card); });

        visible.forEach(function(card, index) {
            card.style.animationDelay = (Math.min(index, 7) * 0.02) + 's';
            card.style.animation = 'none';
            void card.offsetWidth;
            card.style.animation = '';
        });

        const countEl = document.getElementById('clanResultCount');
        if (countEl) countEl.textContent = 'Showing ' + visible.length + ' of ' + allCards.length + ' clans';

        const emptyEl = document.getElementById('clanEmpty');
        if (emptyEl) emptyEl.hidden = visible.length !== 0;
    };

    const initClanPage = function() {
        const directory = document.getElementById('clnDirectory');
        if (!directory) return;

        currentClanCategory = directory.dataset.activeCat || 'all';

        const barSearch = document.getElementById('clanBarSearch');
        if (barSearch) {
            barSearch.oninput = function() {
                if (clanSearchTimer) clearTimeout(clanSearchTimer);
                clanSearchTimer = setTimeout(function() {
                    window.refreshClanView();
                }, 180);
            };
        }

        const sortInput = document.getElementById('clanSortSelect');
        if (sortInput) {
            sortInput.onchange = function() { window.applyClanFilters(); };
        }

        if (window.isClanBrowsing()) {
            window.showClanResults();
        }
    };

    window.updateClanTabIndicator = function() {
        const tabs = document.querySelector('.cln-tabs');
        if (!tabs) return;
        const indicator = tabs.querySelector('.prftabind');
        const activeTab = tabs.querySelector('.prftabbtn.active');
        if (!indicator || !activeTab) return;
        indicator.style.left = activeTab.offsetLeft + 'px';
        indicator.style.width = activeTab.offsetWidth + 'px';
        indicator.style.height = activeTab.offsetHeight + 'px';
        indicator.style.top = activeTab.offsetTop + 'px';
        tabs.classList.add('ready');
    };

    window.switchClanDetailTab = function(name) {
        const tabs = document.querySelector('.cln-tabs');
        if (!tabs) return;

        const target = tabs.querySelector('.prftabbtn[data-cln-tab="' + name + '"]');
        if (!target) return;

        tabs.querySelectorAll('.prftabbtn').forEach(function(tab) {
            const active = tab === target;
            tab.classList.toggle('active', active);
            tab.setAttribute('aria-selected', active ? 'true' : 'false');
            tab.setAttribute('tabindex', active ? '0' : '-1');
        });

        document.querySelectorAll('.cln-tabpane').forEach(function(pane) {
            const active = pane.dataset.clnPane === name;
            pane.hidden = !active;
            if (active) {
                pane.classList.remove('prfpanefade');
                void pane.offsetWidth;
                pane.classList.add('prfpanefade');
            }
        });

        window.updateClanTabIndicator();

        if (name === 'home') {
            requestAnimationFrame(function() {
                document.querySelectorAll('.cln-wall .rctcnt[data-target]').forEach(function(el) {
                    window.clanSetReactionCount(el, parseInt(el.dataset.target, 10) || 0);
                });
            });
        }
    };

    window.filterClanMembers = function() {
        const search = document.getElementById('clnMemberSearch');
        if (!search) return;

        const q = search.value.trim().toLowerCase();
        let shown = 0;

        document.querySelectorAll('#clnRoster .cln-mcard').forEach(function(card) {
            const name = (card.dataset.name || '').toLowerCase();
            const role = (card.dataset.role || '').toLowerCase();
            const match = !q || name.indexOf(q) !== -1 || role.indexOf(q) !== -1;
            card.hidden = !match;
            if (match) shown++;
        });

        const shownEl = document.getElementById('clnMemberShown');
        if (shownEl) shownEl.textContent = shown;

        const emptyEl = document.getElementById('clnMemberEmpty');
        if (emptyEl) emptyEl.hidden = shown !== 0;
    };

    window.clanSetReactionCount = function(el, value) {
        if (!el) return;
        const val = String(Math.max(0, parseInt(value, 10) || 0));
        const digitHeight = 28;
        let strips = el.querySelectorAll('.dgtcnt');

        if (strips.length !== val.length) {
            el.textContent = '';
            for (let i = 0; i < val.length; i++) {
                const cnt = document.createElement('span');
                cnt.className = 'dgtcnt';
                const stp = document.createElement('span');
                stp.className = 'dgtstp';
                for (let j = 0; j <= 9; j++) {
                    const num = document.createElement('span');
                    num.className = 'dgtnum';
                    num.textContent = String(j);
                    stp.appendChild(num);
                }
                cnt.appendChild(stp);
                el.appendChild(cnt);
            }
            strips = el.querySelectorAll('.dgtcnt');
        }

        for (let i = 0; i < val.length; i++) {
            const digit = parseInt(val[i], 10) || 0;
            const strip = strips[i] ? strips[i].querySelector('.dgtstp') : null;
            if (strip) strip.style.transform = 'translateY(-' + (digit * digitHeight) + 'px)';
        }
    };

    const wallState = function() {
        const wall = document.getElementById('clnWall');
        if (!wall) return null;
        const uid = parseInt(wall.dataset.userid, 10) || 0;
        return {
            clanID: parseInt(wall.dataset.clanId, 10) || 0,
            canPost: wall.dataset.canPost === '1',
            username: wall.dataset.username || 'You',
            userID: uid,
            roleName: wall.dataset.roleName || '',
            roleColor: wall.dataset.roleColor || ''
        };
    };

    const wallAvatar = function(uid, name, className) {
        const avt = document.createElement('div');
        avt.className = className;
        const img = document.createElement('img');
        img.src = uid > 0 ? '/api/v1/avatar/headshot/' + uid + '.png' : '/static/useful/temp/pfp.png';
        img.alt = name;
        img.onerror = function() {
            this.onerror = null;
            this.src = '/static/useful/temp/pfp.png';
        };
        avt.appendChild(img);
        return avt;
    };

    const wallAuthorLink = function(uid, name, className) {
        const link = document.createElement('a');
        link.className = className;
        link.textContent = name;
        if (uid > 0) {
            link.href = '/user/' + uid;
            link.setAttribute('hx-get', '/user/' + uid);
            link.setAttribute('hx-target', 'body');
            link.setAttribute('hx-push-url', 'true');
        } else {
            link.href = '#';
        }
        return link;
    };

    const wallRoleBadge = function(state) {
        if (!state.roleName) return null;
        const badge = document.createElement('span');
        badge.className = 'cln-rolebadge';
        badge.textContent = state.roleName;
        if (state.roleColor) badge.style.setProperty('--cln-role-color', state.roleColor);
        return badge;
    };

    const createWallPostNode = function(state, postID, text) {
        const item = document.createElement('div');
        item.className = 'pstitm';
        item.dataset.id = String(postID);
        item.dataset.authorId = String(state.userID);
        item.dataset.author = state.username;

        const card = document.createElement('div');
        card.className = 'pstcrd';
        card.appendChild(wallAvatar(state.userID, state.username, 'pstavt'));

        const hdr = document.createElement('div');
        hdr.className = 'psthdr';
        hdr.appendChild(wallAuthorLink(state.userID, state.username, 'pstaut'));
        const roleBadge = wallRoleBadge(state);
        if (roleBadge) hdr.appendChild(roleBadge);

        const ttp = document.createElement('div');
        ttp.className = 'ttpcnt';
        const time = document.createElement('span');
        time.className = 'psttme';
        time.textContent = 'Just now';
        const full = document.createElement('span');
        full.className = 'ttptxt';
        full.textContent = new Date().toLocaleString();
        ttp.appendChild(time);
        ttp.appendChild(full);
        hdr.appendChild(ttp);
        card.appendChild(hdr);

        const actions = document.createElement('div');
        actions.className = 'pstact';
        if (state.canPost) {
            const react = document.createElement('button');
            react.type = 'button';
            react.className = 'pstrct';
            react.title = 'React';
            react.setAttribute('aria-label', 'React to post');
            const ico = document.createElement('img');
            ico.className = 'pstico';
            ico.src = '/static/useful/icons/fadd.png';
            ico.alt = '';
            react.appendChild(ico);
            react.addEventListener('click', function() {
                window.reactClanWallPost(state.clanID, postID, react);
            });
            actions.appendChild(react);
        }
        const del = document.createElement('button');
        del.type = 'button';
        del.className = 'pstrdel';
        del.title = 'Delete post';
        del.setAttribute('aria-label', 'Delete post');
        const delIco = document.createElement('i');
        delIco.className = 'fa-solid fa-trash';
        delIco.setAttribute('aria-hidden', 'true');
        del.appendChild(delIco);
        del.addEventListener('click', function() {
            window.deleteClanWallPost(state.clanID, postID, del);
        });
        actions.appendChild(del);
        card.appendChild(actions);

        const body = document.createElement('div');
        body.className = 'pbdy';
        body.textContent = text;
        card.appendChild(body);
        item.appendChild(card);

        const badge = document.createElement('div');
        badge.className = 'rctbdg';
        badge.dataset.postId = String(postID);
        if (state.canPost) {
            badge.title = 'React';
            badge.setAttribute('role', 'button');
            badge.setAttribute('tabindex', '0');
            badge.addEventListener('click', function() {
                window.reactClanWallPost(state.clanID, postID, badge);
            });
        } else {
            badge.classList.add('is-locked');
        }
        const badgeIco = document.createElement('div');
        badgeIco.className = 'rctico';
        const face = document.createElement('i');
        face.className = 'fa-regular fa-face-laugh-squint';
        face.setAttribute('aria-hidden', 'true');
        badgeIco.appendChild(face);
        badge.appendChild(badgeIco);
        const count = document.createElement('span');
        count.className = 'rctcnt';
        count.dataset.target = '0';
        badge.appendChild(count);
        window.clanSetReactionCount(count, 0);
        item.appendChild(badge);

        if (window.htmx && typeof window.htmx.process === 'function') {
            window.htmx.process(item);
        }
        return item;
    };

    window.submitClanWallPost = function(event, clanID) {
        if (event) event.preventDefault();
        const form = document.getElementById('clnWallForm');
        const input = document.getElementById('clnWallInput');
        if (!form || !input) return false;

        const text = input.value.trim();
        if (!text) {
            form.classList.add('inerr');
            input.focus();
            setTimeout(function() { form.classList.remove('inerr'); }, 1200);
            return false;
        }

        const send = document.getElementById('clnWallSend');
        const posts = document.getElementById('clnWallPosts');
        const state = wallState();
        if (!state) return false;

        (async function() {
            try {
                const payload = await window.clanPost('/clans/' + clanID + '/wall', {
                    data: { body: text },
                    button: send
                });
                if (!posts) return;
                const empty = posts.querySelector('.feedemt');
                if (empty) empty.remove();
                const node = createWallPostNode(state, payload.id || 0, text);
                posts.insertBefore(node, posts.firstChild);
                input.value = '';
                input.focus();
            } catch (err) {
                window.clanHandleError(err, "Couldn't post that");
            }
        })();
        return false;
    };

    window.reactClanWallPost = function(clanID, postID, el) {
        let badge = null;
        if (el && el.closest) {
            badge = el.closest('.rctbdg') || (el.closest('.pstitm') || el).querySelector('.rctbdg');
        }
        if (!badge) {
            badge = document.querySelector('.cln-wall .pstitm[data-id="' + postID + '"] .rctbdg');
        }
        if (!badge || badge.classList.contains('is-locked') || badge.dataset.busy === '1') return;

        badge.dataset.busy = '1';
        (async function() {
            try {
                const payload = await window.clanPost('/clans/' + clanID + '/wall/' + postID + '/react', {});
                const count = badge.querySelector('.rctcnt');
                badge.classList.toggle('reacted', !!payload.reacted);
                if (count) {
                    count.dataset.target = String(payload.count || 0);
                    window.clanSetReactionCount(count, payload.count || 0);
                }
                badge.classList.remove('rctpop');
                void badge.offsetWidth;
                badge.classList.add('rctpop');
                setTimeout(function() { badge.classList.remove('rctpop'); }, 320);
            } catch (err) {
                window.clanHandleError(err, "Couldn't react");
            } finally {
                delete badge.dataset.busy;
            }
        })();
    };

    window.deleteClanWallPost = function(clanID, postID, btn) {
        const item = btn ? btn.closest('.pstitm') : null;
        window.clanConfirm({
            title: 'Delete this post?',
            message: 'This removes the post and its reactions from the clan wall.',
            confirmLabel: 'Delete',
            danger: true,
            onConfirm: function() {
                (async function() {
                    try {
                        await window.clanPost('/clans/' + clanID + '/wall/' + postID + '/delete', { button: btn });
                        if (item) {
                            item.classList.add('is-removing');
                            setTimeout(function() {
                                item.remove();
                                const posts = document.getElementById('clnWallPosts');
                                if (posts && !posts.querySelector('.pstitm')) {
                                    const empty = document.createElement('div');
                                    empty.className = 'feedemt';
                                    empty.textContent = 'No wall posts yet. Be the first to say something.';
                                    posts.appendChild(empty);
                                }
                            }, 180);
                        }
                    } catch (err) {
                        window.clanHandleError(err, "Couldn't delete that post");
                    }
                })();
            }
        });
    };

    window.joinClan = function(clanID, btn) {
        (async function() {
            try {
                const payload = await window.clanPost('/clans/' + clanID + '/join', { button: btn });
                if (payload.status === 'requested') {
                    window.clanRefreshWithNotice(window.location.pathname, 'Request sent. An admin will review it.');
                } else {
                    window.clanRefreshWithNotice(window.location.pathname, 'Welcome to the clan.');
                }
            } catch (err) {
                window.clanHandleError(err, "Couldn't join this clan");
            }
        })();
    };

    window.leaveClan = function(clanID, btn) {
        window.clanConfirm({
            title: 'Leave this clan?',
            message: "You'll lose your role and will need to join again to get back in.",
            confirmLabel: 'Leave',
            danger: true,
            onConfirm: function() {
                (async function() {
                    try {
                        await window.clanPost('/clans/' + clanID + '/leave', { button: btn });
                        window.clanRefreshWithNotice(window.location.pathname, 'You left the clan.');
                    } catch (err) {
                        window.clanHandleError(err, "Couldn't leave this clan");
                    }
                })();
            }
        });
    };

    window.cancelClanRequest = function(clanID, btn) {
        window.clanConfirm({
            title: 'Cancel your request?',
            message: 'The clan admins will no longer see your join request.',
            confirmLabel: 'Cancel request',
            onConfirm: function() {
                (async function() {
                    try {
                        await window.clanPost('/clans/' + clanID + '/request/cancel', { button: btn });
                        window.clanRefreshWithNotice(window.location.pathname, 'Request cancelled.');
                    } catch (err) {
                        window.clanHandleError(err, "Couldn't cancel your request");
                    }
                })();
            }
        });
    };

    window.focusClanJoinRequest = function(clanID, btn) {
        const form = document.getElementById('clnJoinRequestForm');
        const area = document.getElementById('clnJoinMessage');
        if (!form) {
            if (clanID) window.joinClan(clanID, btn);
            return;
        }
        form.scrollIntoView({ behavior: 'smooth', block: 'center' });
        if (area) setTimeout(function() { area.focus({ preventScroll: true }); }, 260);
    };

    window.submitClanJoinRequest = function(event, clanID) {
        if (event) event.preventDefault();
        const form = document.getElementById('clnJoinRequestForm');
        const area = document.getElementById('clnJoinMessage');
        const btn = document.getElementById('clnJoinRequestBtn');
        if (!form) return false;

        const message = area ? area.value.trim() : '';
        (async function() {
            try {
                await window.clanPost('/clans/' + clanID + '/join', {
                    data: { message: message },
                    button: btn
                });
                window.clanRefreshWithNotice(window.location.pathname, 'Request sent. An admin will review it.');
            } catch (err) {
                window.clanHandleError(err, "Couldn't send your request");
            }
        })();
        return false;
    };

    const initClanDetail = function() {
        const tabs = document.querySelector('.cln-tabs');
        if (tabs) {
            setTimeout(window.updateClanTabIndicator, 40);
            setTimeout(window.updateClanTabIndicator, 180);

            if (tabs.dataset.clnBound !== '1') {
                tabs.dataset.clnBound = '1';
                tabs.addEventListener('keydown', function(e) {
                    if (!e.target.classList.contains('prftabbtn')) return;
                    const buttons = Array.prototype.slice.call(tabs.querySelectorAll('.prftabbtn'));
                    const current = buttons.indexOf(e.target);
                    let next = current;

                    if (e.key === 'ArrowRight') next = (current + 1) % buttons.length;
                    else if (e.key === 'ArrowLeft') next = (current - 1 + buttons.length) % buttons.length;
                    else if (e.key === 'Home') next = 0;
                    else if (e.key === 'End') next = buttons.length - 1;
                    else return;

                    e.preventDefault();
                    window.switchClanDetailTab(buttons[next].dataset.clnTab);
                    buttons[next].focus();
                });
            }
        }

        const memberSearch = document.getElementById('clnMemberSearch');
        if (memberSearch && memberSearch.dataset.clnBound !== '1') {
            memberSearch.dataset.clnBound = '1';
            memberSearch.addEventListener('input', window.filterClanMembers);
            if (memberSearch.value) window.filterClanMembers();
        }

        document.querySelectorAll('.cln-wall .rctcnt[data-target]').forEach(function(el) {
            window.clanSetReactionCount(el, parseInt(el.dataset.target, 10) || 0);
        });
    };

    const initWallDelegation = function() {
        if (window.__clnWallDelegationBound) return;
        window.__clnWallDelegationBound = true;

        document.addEventListener('keydown', function(e) {
            if (e.key !== 'Enter' && e.key !== ' ') return;
            const badge = e.target.closest ? e.target.closest('.cln-wall .rctbdg[role="button"]') : null;
            if (!badge) return;
            e.preventDefault();
            badge.click();
        });
    };

    initClanPage();
    initClanDetail();
    initWallDelegation();

    document.addEventListener('htmx:afterSettle', function() {
        initClanPage();
        initClanDetail();
    });

    if (!window.__clnDetailResizeBound) {
        window.__clnDetailResizeBound = true;
        window.addEventListener('resize', function() {
            if (document.querySelector('.cln-tabs')) window.updateClanTabIndicator();
        });
    }
})();
