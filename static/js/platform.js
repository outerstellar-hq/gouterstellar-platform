document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('.toast').forEach(function (toast) {
        setTimeout(function () { toast.remove(); }, 5000);
    });

    var themeToggle = document.getElementById('theme-toggle');
    if (themeToggle) {
        themeToggle.addEventListener('click', function () {
            var html = document.documentElement;
            var current = html.getAttribute('data-theme');
            var next = current === 'dark' ? 'light' : 'dark';
            html.setAttribute('data-theme', next);
            fetch('/settings/preferences', {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: 'theme=' + next
            });
        });
    }

    var csrfMeta = document.querySelector('meta[name="csrf-token"]');
    if (csrfMeta) {
        var csrfToken = csrfMeta.getAttribute('content');
        var originalFetch = window.fetch;
        window.fetch = function (url, options) {
            options = options || {};
            options.headers = options.headers || {};
            if (options.method && options.method !== 'GET') {
                options.headers['X-CSRF-Token'] = csrfToken;
            }
            return originalFetch(url, options);
        };
    }

    document.querySelectorAll('[data-confirm]').forEach(function (el) {
        el.addEventListener('click', function (e) {
            if (!confirm(el.getAttribute('data-confirm'))) {
                e.preventDefault();
            }
        });
    });

    document.querySelectorAll('[data-copy-target]').forEach(function (button) {
        button.addEventListener('click', function () {
            var targetID = button.getAttribute('data-copy-target');
            var target = targetID ? document.getElementById(targetID) : null;
            if (!target) {
                return;
            }
            var statusID = button.getAttribute('data-copy-status');
            var status = statusID ? document.getElementById(statusID) : null;
            var value = target.value || target.textContent || '';
            var copied = function () {
                if (status) {
                    status.textContent = 'Copied';
                }
            };
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(value).then(copied, function () {
                    copyBySelection(target);
                    copied();
                });
                return;
            }
            copyBySelection(target);
            copied();
        });
    });
});

function copyBySelection(target) {
    if (target.select) {
        target.select();
        target.setSelectionRange(0, target.value.length);
    }
    document.execCommand('copy');
}
