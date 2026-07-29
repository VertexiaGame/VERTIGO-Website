let adminUptimeTimer = null;
let adminPollTimer = null;
let currentAdminUserPage = 1;

if (typeof window.escapeHTML !== 'function') {
    window.escapeHTML = function(str) {
        if (!str) return '';
        return String(str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
    };
}

window.updateAdminTabIndicator = function() {
    const indicator = document.querySelector('.tabcnt .tabind');
    const activeTab = document.querySelector('.tabcnt .tabbtn.active');
    if (indicator && activeTab) {
        indicator.style.left = activeTab.offsetLeft + 'px';
        indicator.style.width = activeTab.offsetWidth + 'px';
        indicator.style.height = activeTab.offsetHeight + 'px';
        indicator.style.top = activeTab.offsetTop + 'px';
    }
};

window.updateAdminViewportHeight = function() {
    const activeContent = document.querySelector('.settabcntnt.active');
    const viewport = document.querySelector('.settabvwp');
    if (activeContent && viewport) {
        viewport.style.height = activeContent.offsetHeight + 'px';
        if (window.adminHeightTimeout) {
            clearTimeout(window.adminHeightTimeout);
        }
        window.adminHeightTimeout = setTimeout(() => {
            viewport.style.height = '';
        }, 350);
    }
};

window.switchAdminTab = function(tabName, event) {
    const tabs = Array.from(document.querySelectorAll('.tabcnt .tabbtn'));
    const clickedTab = event ? event.currentTarget : null;
    if (!clickedTab || clickedTab.classList.contains('active')) return;

    const currentContent = document.querySelector('.settabcntnt.active');
    const targetContent = document.getElementById('tab-' + tabName);
    if (!targetContent) return;

    tabs.forEach(tab => tab.classList.remove('active'));
    clickedTab.classList.add('active');

    window.updateAdminTabIndicator();

    const viewport = document.querySelector('.settabvwp');
    const currentHeight = currentContent ? currentContent.offsetHeight : 0;

    if (viewport) {
        viewport.style.height = currentHeight + 'px';
        viewport.offsetHeight;

        if (currentContent) {
            currentContent.classList.remove('active');
        }
        targetContent.classList.add('active');

        window.updateAdminViewportHeight();
    }
};

const formatUptimeJS = (seconds) => {
    if (seconds < 0) seconds = 0;
    const days = Math.floor(seconds / 86400);
    seconds %= 86400;
    const hours = Math.floor(seconds / 3600);
    seconds %= 3600;
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;

    if (days > 0) return `${days}d ${hours}h ${mins}m ${secs}s`;
    if (hours > 0) return `${hours}h ${mins}m ${secs}s`;
    if (mins > 0) return `${mins}m ${secs}s`;
    return `${secs}s`;
};

const updateLiveUptime = () => {
    const el = document.getElementById('admUptime');
    if (!el) return;
    const startMs = parseInt(el.dataset.startTimeMs, 10);
    if (!startMs) return;

    const diffSec = Math.floor((Date.now() - startMs) / 1000);
    el.textContent = formatUptimeJS(diffSec);
};

const pollAdminStatus = () => {
    fetch('/api/v1/admin/status')
        .then(res => res.json())
        .then(data => {
            if (!data || data.error) return;
            const cpuEl = document.getElementById('admCpuUsage');
            if (cpuEl) cpuEl.textContent = data.cpu_usage;

            const allocEl = document.getElementById('admMemAlloc');
            if (allocEl) allocEl.textContent = data.mem_alloc;

            const sysEl = document.getElementById('admMemSys');
            if (sysEl) sysEl.textContent = data.mem_sys;

            const hostEl = document.getElementById('admHostMem');
            if (hostEl) hostEl.textContent = `${data.sys_mem_used} / ${data.sys_mem_total} (${data.sys_mem_pct})`;

            const goEl = document.getElementById('admGoroutines');
            if (goEl) goEl.textContent = data.goroutines;

            const gcEl = document.getElementById('admGcCycles');
            if (gcEl) gcEl.textContent = data.gc_cycles;
        })
        .catch(() => {});
};

window.fetchAdminUsers = function(page = 1) {
    currentAdminUserPage = page;
    const searchInput = document.getElementById('admUserSearch');
    const roleSelect = document.getElementById('admRoleFilter');
    const q = searchInput ? searchInput.value.trim() : '';
    const role = roleSelect ? roleSelect.value : '-1';

    const grid = document.getElementById('admUsersGrid');
    if (!grid) return;

    fetch(`/api/v1/admin/users?q=${encodeURIComponent(q)}&role=${role}&page=${page}&limit=15`)
        .then(res => res.json())
        .then(data => {
            if (!data || data.error) return;
            window.renderAdminUserTable(data);
        })
        .catch(() => {});
};

window.renderAdminUserTable = function(data) {
    const grid = document.getElementById('admUsersGrid');
    const pgnTxt = document.getElementById('admPgnTxt');
    const prevBtn = document.getElementById('admPrevBtn');
    const nextBtn = document.getElementById('admNextBtn');

    if (!grid) return;

    const page = data.page || 1;
    const totalPages = Math.max(1, data.total_pages || 1);

    if (!data.users || data.users.length === 0) {
        grid.innerHTML = '<div class="frnemt adm-emt">No users found</div>';
        if (pgnTxt) pgnTxt.textContent = `Page ${page} of ${totalPages}`;
        if (prevBtn) prevBtn.disabled = (page <= 1);
        if (nextBtn) nextBtn.disabled = (page >= totalPages);
        window.updateAdminViewportHeight();
        return;
    }

    let html = '';
    data.users.forEach(u => {
        const roleClass = u.role_name === 'head manager' ? 'head-manager' : (u.role_name === 'vertexia-team' ? 'vertexia-team' : u.role_name);
        const safeUsername = window.escapeHTML(u.username);
        const safeDisplayName = window.escapeHTML(u.display_name);

        html += `
            <div class="frncrd">
                <div class="frntop">
                    <a href="/user/${u.id}" hx-get="/user/${u.id}" hx-target="body" hx-push-url="true" class="frnavt">
                        <img src="/api/v1/avatar/headshot/${u.id}.png" onerror="this.src='/static/useful/temp/pfp.png';" alt="${safeUsername}">
                    </a>
                    <div class="frndtl">
                        <a href="/user/${u.id}" hx-get="/user/${u.id}" hx-target="body" hx-push-url="true" class="frndn">${safeDisplayName}</a>
                        <span class="frnun">@${safeUsername} • #${u.id}</span>
                    </div>
                </div>
                <div class="adm-crdmeta">
                    <span class="adm-badge adm-badge-${roleClass}">${window.escapeHTML(u.role_display)}</span>
                    <span>Joined ${u.creation_date}</span>
                </div>
                <div class="frnact">
                    <button type="button" class="happy hpyprim hpyinl hpysm" onclick="window.viewAdminUser(${u.id})">
                        <span>View User</span>
                    </button>
                </div>
            </div>
        `;
    });

    grid.innerHTML = html;

    if (pgnTxt) {
        pgnTxt.textContent = `Page ${page} of ${totalPages}`;
    }

    if (prevBtn) prevBtn.disabled = (page <= 1);
    if (nextBtn) nextBtn.disabled = (page >= totalPages);

    window.updateAdminViewportHeight();
};

window.viewAdminUser = function(userId) {
    if (window.htmx) {
        htmx.ajax('GET', `/admin/users/${userId}`, { target: 'body', pushUrl: true });
    } else {
        window.location.href = `/admin/users/${userId}`;
    }
};

const initAdminPage = () => {
    if (adminUptimeTimer) clearInterval(adminUptimeTimer);
    if (adminPollTimer) clearInterval(adminPollTimer);

    setTimeout(() => {
        window.updateAdminTabIndicator();
        window.updateAdminViewportHeight();
    }, 50);
    setTimeout(() => {
        window.updateAdminTabIndicator();
        window.updateAdminViewportHeight();
    }, 150);

    updateLiveUptime();
    adminUptimeTimer = setInterval(updateLiveUptime, 1000);
    adminPollTimer = setInterval(pollAdminStatus, 5000);
};

initAdminPage();

document.addEventListener('htmx:afterSettle', initAdminPage);
document.addEventListener('htmx:beforeTransition', () => {
    if (adminUptimeTimer) clearInterval(adminUptimeTimer);
    if (adminPollTimer) clearInterval(adminPollTimer);
});

window.addEventListener('resize', () => {
    window.updateAdminTabIndicator();
    window.updateAdminViewportHeight();
});