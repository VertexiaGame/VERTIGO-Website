(function() {
    if (window.__vtxCommentsBound) return;
    window.__vtxCommentsBound = true;

    function root() { return document.getElementById('shpitmComments'); }

    function el(tag, cls, text) {
        var e = document.createElement(tag);
        if (cls) e.className = cls;
        if (text != null) e.textContent = text;
        return e;
    }

    function userLink(cls, id, text) {
        var a = el('a', cls, text);
        a.href = '/user/' + id;
        a.setAttribute('hx-get', '/user/' + id);
        a.setAttribute('hx-target', 'body');
        a.setAttribute('hx-push-url', 'true');
        return a;
    }

    function avatar(cls, c) {
        var box = el('div', cls);
        var img = document.createElement('img');
        img.src = '/api/v1/avatar/headshot/' + Number(c.user_id) + '.png';
        img.alt = c.username;
        img.onerror = function() { this.onerror = null; this.src = '/static/useful/temp/pfp.png'; };
        box.appendChild(img);
        return box;
    }

    function actions(cls, c, canPost) {
        var act = el('div', cls);
        if (canPost) {
            var rpl = el('div', 'pstrpl');
            rpl.title = 'Reply';
            rpl.dataset.commentReply = c.id;
            rpl.appendChild(el('i', 'fa-solid fa-reply'));
            act.appendChild(rpl);
        }
        if (c.can_delete) {
            var del = el('a', 'pstflg shpcmt-del');
            del.href = '#';
            del.title = 'Remove';
            del.dataset.commentRemove = c.id;
            del.appendChild(el('i', 'fa-solid fa-trash'));
            act.appendChild(del);
        }
        return act;
    }

    function timeSpan(cls, c) {
        var t = el('span', cls, c.time_ago || 'just now');
        if (c.full_date) t.title = c.full_date;
        return t;
    }

    function buildReply(c, parent, canPost) {
        var item = el('div', 'cmtitm');
        item.id = 'comment-' + c.id;
        item.dataset.commentId = c.id;
        item.dataset.userId = c.user_id;
        item.dataset.username = c.username;
        var card = el('div', 'cmtcrd');
        card.appendChild(avatar('cmtavt', c));
        var hdr = el('div', 'cmthdr');
        hdr.appendChild(userLink('cmtaut', c.user_id, c.username));
        hdr.appendChild(timeSpan('cmttme', c));
        card.appendChild(hdr);
        var rep = el('div', 'cmtrep');
        rep.appendChild(el('i', 'fa-solid fa-reply'));
        rep.appendChild(el('span', '', 'replying to'));
        rep.appendChild(userLink('', parent.userId, '@' + parent.username));
        card.appendChild(rep);
        card.appendChild(actions('cmtact', c, canPost));
        card.appendChild(el('div', 'cmtbdy', c.body));
        item.appendChild(card);
        return item;
    }

    function buildComment(c, canPost) {
        var item = el('div', 'pstitm');
        item.id = 'comment-' + c.id;
        item.dataset.commentId = c.id;
        item.dataset.userId = c.user_id;
        item.dataset.username = c.username;
        var card = el('div', 'pstcrd');
        card.appendChild(avatar('pstavt', c));
        var hdr = el('div', 'psthdr');
        hdr.appendChild(userLink('pstaut', c.user_id, c.username));
        hdr.appendChild(timeSpan('psttme', c));
        card.appendChild(hdr);
        card.appendChild(actions('pstact', c, canPost));
        card.appendChild(el('div', 'pbdy', c.body));
        item.appendChild(card);
        var nest = el('div', 'cmtnest shpcmt-nest');
        var parent = { userId: c.user_id, username: c.username };
        (c.replies || []).forEach(function(r) { nest.appendChild(buildReply(r, parent, canPost)); });
        nest.hidden = !(c.replies && c.replies.length);
        item.appendChild(nest);
        return item;
    }

    function showError(msg) {
        var box = document.getElementById('shpitmCommentError');
        if (!box) return;
        var composer = document.getElementById('shpitmFeedBox');
        clearTimeout(box.dismissTimer);
        cancelAnimationFrame(box.showFrame);
        box.textContent = msg || '';
        if (composer) composer.classList.toggle('inerr', !!msg);
        if (!msg) {
            box.classList.remove('visible');
            return;
        }
        box.showFrame = requestAnimationFrame(function() { box.classList.add('visible'); });
        box.dismissTimer = setTimeout(function() {
            box.classList.remove('visible');
            if (composer) composer.classList.remove('inerr');
        }, 4000);
    }

    function setReplyTarget(id, username) {
        var parent = document.getElementById('shpitmCommentParent');
        var chip = document.getElementById('shpitmReplyTo');
        var name = document.getElementById('shpitmReplyToName');
        var input = document.getElementById('shpitmCommentInput');
        if (!parent || !chip) return;
        parent.value = id ? String(id) : '';
        chip.hidden = !id;
        if (name) {
            name.textContent = id ? '@' + username : '';
            name.href = id ? '#comment-' + id : '#';
        }
        document.querySelectorAll('#shpitmComments .pstrpl.active').forEach(function(b) { b.classList.remove('active'); });
        if (id) {
            var btn = document.querySelector('#shpitmComments [data-comment-reply="' + id + '"]');
            if (btn) btn.classList.add('active');
        }
        if (input) input.focus();
    }

    async function send(url, form) {
        var res = await fetch(url, {
            method: 'POST',
            body: form,
            headers: { 'Accept': 'application/json', 'X-Requested-With': 'XMLHttpRequest' },
            credentials: 'same-origin'
        });
        var data = {};
        try { data = await res.json(); } catch (e) {}
        if (!res.ok) throw new Error(data.message || data.error || 'Something went wrong, try again.');
        return data;
    }

    var posting = false;
    async function postComment(composer) {
        var wrap = root();
        var input = document.getElementById('shpitmCommentInput');
        var list = document.getElementById('shpitmCommentsList');
        if (!wrap || !input || !list || posting) return;
        var text = (input.value || '').trim();
        if (text.length < 3) {
            input.focus();
            showError(text ? 'comment needs to be between 3 and 500 characters' : '');
            return;
        }
        var form = new FormData(composer);
        form.set('body', text.slice(0, 500));
        posting = true;
        try {
            var data = await send(composer.action, form);
            if (!wrap.isConnected) return;
            var c = data.comment;
            if (!c) throw new Error('The server did not confirm your comment. Refresh before trying again.');
            showError('');
            var empty = list.querySelector(':scope > .feedemt');
            if (empty) empty.remove();
            var node;
            if (c.parent_id) {
                var parentItem = document.getElementById('comment-' + c.parent_id);
                var nest = parentItem && parentItem.querySelector('.shpcmt-nest');
                if (nest) {
                    node = buildReply(c, { userId: parentItem.dataset.userId, username: parentItem.dataset.username }, true);
                    nest.hidden = false;
                    nest.appendChild(node);
                }
            }
            if (!node) {
                node = buildComment(c, true);
                list.insertAdjacentElement('afterbegin', node);
            }
            input.value = '';
            setReplyTarget(0);
            if (window.htmx) window.htmx.process(node);
            node.scrollIntoView({ block: 'nearest' });
        } catch (err) {
            if (wrap.isConnected) showError(err.message);
        } finally {
            posting = false;
        }
    }

    document.addEventListener('submit', function(e) {
        if (e.target.id !== 'shpitmFeedBox') return;
        e.preventDefault();
        postComment(e.target);
    });

    async function removeComment(id) {
        var wrap = root();
        if (!wrap) return;
        var form = new FormData();
        form.set('csrf', wrap.dataset.csrf || '');
        try {
            await send('/comments/' + Number(id) + '/remove', form);
            var node = document.getElementById('comment-' + id);
            if (node) {
                var nest = node.parentElement;
                node.remove();
                if (nest && nest.classList.contains('shpcmt-nest') && !nest.children.length) nest.hidden = true;
            }
            var parent = document.getElementById('shpitmCommentParent');
            if (parent && parent.value && !document.getElementById('comment-' + parent.value)) setReplyTarget(0);
            var list = document.getElementById('shpitmCommentsList');
            if (list && !list.children.length) list.appendChild(el('div', 'feedemt', 'nobody has commented yet... be the first!'));
            showError('');
        } catch (err) {
            showError(err.message);
        }
    }

    async function loadMore(btn) {
        var wrap = root();
        var list = document.getElementById('shpitmCommentsList');
        if (!wrap || !list || btn.disabled) return;
        btn.disabled = true;
        try {
            var url = '/api/v1/comments/' + encodeURIComponent(wrap.dataset.commentsType) + '/' + Number(wrap.dataset.commentsId) + '?before=' + Number(btn.dataset.before);
            var res = await fetch(url, { headers: { 'Accept': 'application/json' }, credentials: 'same-origin' });
            var data = await res.json();
            if (!res.ok) throw new Error(data.error || 'Comments could not be loaded.');
            var canPost = !!document.getElementById('shpitmFeedBox');
            (data.comments || []).forEach(function(c) {
                if (!document.getElementById('comment-' + c.id)) list.appendChild(buildComment(c, canPost));
            });
            var last = (data.comments || []).slice(-1)[0];
            if (data.has_more && last) {
                btn.dataset.before = last.id;
                btn.disabled = false;
            } else {
                btn.remove();
            }
        } catch (err) {
            btn.disabled = false;
            showError(err.message);
        }
    }

    document.addEventListener('click', function(e) {
        if (!root()) return;
        var reply = e.target.closest('#shpitmComments [data-comment-reply]');
        if (reply) {
            e.preventDefault();
            var item = reply.closest('[data-comment-id]');
            if (reply.classList.contains('active')) setReplyTarget(0);
            else setReplyTarget(reply.dataset.commentReply, item ? item.dataset.username : '');
            return;
        }
        var del = e.target.closest('#shpitmComments [data-comment-remove]');
        if (del) {
            e.preventDefault();
            if (!del.classList.contains('confirm')) {
                del.classList.add('confirm');
                del.title = 'Click again to remove';
                setTimeout(function() { del.classList.remove('confirm'); del.title = 'Remove'; }, 3000);
                return;
            }
            removeComment(del.dataset.commentRemove);
            return;
        }
        if (e.target.closest('#shpitmReplyCancel')) {
            e.preventDefault();
            setReplyTarget(0);
            return;
        }
        var more = e.target.closest('#shpitmCommentsMore');
        if (more) {
            e.preventDefault();
            loadMore(more);
        }
    });
})();
