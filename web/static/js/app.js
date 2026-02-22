// Linux Router GUI - JavaScript utilities

// ============================================
// Global Confirmation Modal System
// ============================================
var pendingAction = null;
var pendingMethod = 'POST';
var pendingBody = null;

// Event delegation for confirm buttons - works with HTMX content swaps
document.addEventListener('click', function(event) {
    var target = event.target.closest('[data-confirm-action]');
    if (target) {
        event.preventDefault();
        var action = target.getAttribute('data-confirm-action');
        var method = target.getAttribute('data-confirm-method') || 'POST';
        var body = target.getAttribute('data-confirm-body') || null;
        var message = target.getAttribute('data-confirm-message') || 'Are you sure?';
        
        if (action) {
            showConfirmModal(message, action, method, body);
        }
    }
});

function showConfirmModal(message, actionUrl, method, body) {
    // Support both modal patterns:
    // 1. confirm-modal with confirm-modal-message (standard)
    // 2. confirmModal with confirmTitle and confirmMessage (custom)
    var msgEl = document.getElementById('confirm-modal-message');
    var modal = document.getElementById('confirm-modal');
    
    if (msgEl) {
        msgEl.textContent = message;
    } else {
        // Try custom modal pattern
        var titleEl = document.getElementById('confirmTitle');
        var customMsgEl = document.getElementById('confirmMessage');
        if (titleEl && customMsgEl) {
            titleEl.textContent = 'Confirm Action';
            customMsgEl.textContent = message;
        }
    }
    
    if (modal) {
        modal.classList.remove('hidden');
    } else {
        // Try custom modal
        var customModal = document.getElementById('confirmModal');
        if (customModal) {
            customModal.classList.remove('hidden');
        }
    }
    
    // Set both old and new variables for compatibility
    pendingAction = actionUrl;
    pendingMethod = method || 'POST';
    pendingBody = body || null;
    modalActionUrl = actionUrl;
    modalActionMethod = method || 'POST';
    
    // Also set pendingDeleteUrl for backward compatibility with custom confirmAction implementations
    window.pendingDeleteUrl = actionUrl;
}

function closeConfirmModal() {
    var modal = document.getElementById('confirm-modal');
    if (modal) {
        modal.classList.add('hidden');
    } else {
        // Try custom modal
        var customModal = document.getElementById('confirmModal');
        if (customModal) {
            customModal.classList.add('hidden');
        }
    }
    pendingAction = null;
    pendingBody = null;
}

function confirmAction() {
    if (pendingAction) {
        var options = { method: pendingMethod, credentials: 'same-origin' };
        if (pendingBody) {
            options.headers = {'Content-Type': 'application/x-www-form-urlencoded'};
            options.body = pendingBody;
        }
        fetch(pendingAction, options)
            .then(function(response) { return response.text(); })
            .then(function(html) {
                var alertContainer = document.getElementById('alert-container');
                if (alertContainer) {
                    alertContainer.innerHTML = html;
                }
                htmx.trigger(document.body, 'refresh');
            });
    }
    closeConfirmModal();
}

// ============================================
// Firewall-specific functions
// ============================================
function openAddRuleModal(chain) {
    var chainSelect = document.getElementById('rule-chain');
    if (chainSelect) {
        chainSelect.value = chain;
    }
    var modal = document.getElementById('add-rule-modal');
    if (modal) {
        modal.classList.remove('hidden');
    }
}

function showChain(chainName) {
    // Get current table from URL
    var urlParams = new URLSearchParams(window.location.search);
    var currentTable = urlParams.get('table') || 'filter';
    if (!chainName) {
        window.location.href = '/firewall?table=' + currentTable;
    } else {
        window.location.href = '/firewall?table=' + currentTable + '&chain=' + encodeURIComponent(chainName);
    }
}

// Generic setPolicy - baseUrl should be like '/firewall/chains' or '/netns/name/firewall/chains'
function setPolicy(baseUrl, table, chain, policy, selectEl) {
    if (!policy) return;
    if (selectEl) selectEl.value = '';
    showConfirmModal('Set policy for ' + chain + ' to ' + policy + '?', baseUrl + '/' + chain + '/policy', 'PUT', 'table=' + table + '&policy=' + policy);
}

// Show confirmation modal for deleting a routing table (default namespace)
function showDeleteTableModal(name) {
    showActionModalWithUrl('Are you sure you want to delete routing table "' + name + '"? Any routes in this table will also be deleted.', '/rules/tables/' + encodeURIComponent(name), 'DELETE');
}

// Show confirmation modal for deleting a routing table in a specific namespace
function showDeleteNetnsTableModal(namespace, name) {
    showActionModalWithUrl('Are you sure you want to delete routing table "' + name + '"? Any routes in this table will also be deleted.', '/netns/' + encodeURIComponent(namespace) + '/rules/tables/' + encodeURIComponent(name), 'DELETE');
}

// Helper function to show modal with direct URL (for app.js functions)
function showActionModalWithUrl(message, actionUrl, method) {
    modalActionUrl = actionUrl;
    modalActionMethod = method || 'DELETE';
    
    var msgEl = document.getElementById('confirm-modal-message');
    var modal = document.getElementById('confirm-modal');
    
    if (msgEl) {
        msgEl.textContent = message;
    } else {
        // Try custom modal pattern
        var titleEl = document.getElementById('confirmTitle');
        var customMsgEl = document.getElementById('confirmMessage');
        if (titleEl && customMsgEl) {
            titleEl.textContent = 'Confirm Action';
            customMsgEl.textContent = message;
        }
    }
    
    if (modal) {
        modal.classList.remove('hidden');
    } else {
        // Try custom modal
        var customModal = document.getElementById('confirmModal');
        if (customModal) {
            customModal.classList.remove('hidden');
        }
    }
}

// Refresh the table select dropdown after creating a new table
function refreshTableSelect() {
    // Check if we're in a namespace context by looking at the URL
    var match = window.location.pathname.match(/^\/netns\/([^\/]+)\/rules/);
    var url = '/rules/tables/options';
    if (match) {
        url = '/netns/' + encodeURIComponent(match[1]) + '/rules/tables/options';
    }

    fetch(url, { credentials: 'same-origin' })
        .then(function(response) { return response.text(); })
        .then(function(html) {
            var select = document.getElementById('table-select');
            if (select) {
                select.innerHTML = html;
            }
        })
        .catch(function(err) {
            console.error('Failed to refresh table options:', err);
        });
}

// Listen for refreshTables event to update the dropdown
document.addEventListener('DOMContentLoaded', function() {
    document.body.addEventListener('refreshTables', refreshTableSelect);
});

// ============================================
// Auto-dismiss alerts after 5 seconds
// ============================================
document.addEventListener('htmx:afterSwap', function(event) {
    const alerts = event.detail.target.querySelectorAll('#alert-box');
    alerts.forEach(function(alert) {
        setTimeout(function() {
            alert.style.transition = 'opacity 300ms ease-out';
            alert.style.opacity = '0';
            setTimeout(function() {
                alert.remove();
            }, 300);
        }, 5000);
    });
});

// Confirm dangerous actions
document.addEventListener('htmx:confirm', function(event) {
    if (event.detail.elt.hasAttribute('data-confirm')) {
        event.preventDefault();
        if (confirm(event.detail.elt.getAttribute('data-confirm'))) {
            event.detail.issueRequest();
        }
    }
});

// Handle form validation
document.addEventListener('submit', function(event) {
    const form = event.target;
    if (!form.checkValidity()) {
        event.preventDefault();
        form.reportValidity();
    }
});

// Toggle mobile menu
function toggleMobileMenu() {
    const menu = document.getElementById('mobile-menu');
    if (menu) {
        menu.classList.toggle('hidden');
    }
}

// Copy to clipboard
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(function() {
        showToast('Copied to clipboard');
    }).catch(function(err) {
        console.error('Failed to copy: ', err);
    });
}

// Show toast notification
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `fixed bottom-4 right-4 px-4 py-2 rounded-md text-white text-sm font-medium shadow-lg z-50 ${
        type === 'error' ? 'bg-red-600' :
        type === 'success' ? 'bg-green-600' :
        'bg-gray-800'
    }`;
    toast.textContent = message;
    document.body.appendChild(toast);

    setTimeout(function() {
        toast.style.transition = 'opacity 300ms ease-out';
        toast.style.opacity = '0';
        setTimeout(function() {
            toast.remove();
        }, 300);
    }, 3000);
}

// Format bytes to human readable
function formatBytes(bytes, decimals = 2) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

// Periodic refresh for stats (when element is present)
document.addEventListener('DOMContentLoaded', function() {
    const statsContainer = document.getElementById('stats-container');
    if (statsContainer && statsContainer.hasAttribute('hx-get')) {
        // HTMX will handle the polling via hx-trigger
    }
});

// Handle keyboard shortcuts
document.addEventListener('keydown', function(event) {
    // Escape to close modals
    if (event.key === 'Escape') {
        const modals = document.querySelectorAll('[role="dialog"]');
        modals.forEach(function(modal) {
            const closeBtn = modal.querySelector('[data-close-modal]');
            if (closeBtn) {
                closeBtn.click();
            }
        });
    }
});

// HTMX extension for handling redirects
document.addEventListener('htmx:beforeSwap', function(event) {
    // If server sends HX-Redirect, let HTMX handle it
    if (event.detail.xhr.getResponseHeader('HX-Redirect')) {
        return;
    }
});

// Loading state management - use htmx:ready to ensure HTMX is fully loaded
function initLoadingState() {
    document.addEventListener('htmx:beforeRequest', function(event) {
        var target = event.detail.elt;
        if (target.tagName === 'BUTTON' && !target.hasAttribute('data-no-loading')) {
            target.disabled = true;
            target.setAttribute('data-original-text', target.innerHTML);
            target.innerHTML = '<svg class="animate-spin h-4 w-4 mr-2 inline" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>Loading...';
        }
    });

    function restoreButton(target) {
        if (target && target.tagName === 'BUTTON' && target.hasAttribute('data-original-text')) {
            target.disabled = false;
            target.innerHTML = target.getAttribute('data-original-text');
            target.removeAttribute('data-original-text');
        }
    }

    document.addEventListener('htmx:afterRequest', function(event) {
        restoreButton(event.detail.elt);
    });

    document.addEventListener('htmx:responseError', function(event) {
        restoreButton(event.detail.elt);
    });

    document.addEventListener('htmx:sendError', function(event) {
        restoreButton(event.detail.elt);
    });
}

// Initialize immediately if HTMX is already loaded, otherwise wait for it
if (typeof htmx !== 'undefined') {
    initLoadingState();
} else {
    document.addEventListener('DOMContentLoaded', initLoadingState);
}
