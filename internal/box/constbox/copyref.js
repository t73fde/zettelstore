/*-----------------------------------------------------------------------------
 * Copyright (c) 2026-present Detlef Stern
 *
 * This file is part of Zettelstore.
 *
 * Zettelstore is licensed under the latest version of the EUPL (European Union
 * Public License). Please see file LICENSE.txt for your rights and obligations
 * under this license.
 *
 * SPDX-License-Identifier: EUPL-1.2
 * SPDX-FileCopyrightText: 2026-present Detlef Stern
 *-----------------------------------------------------------------------------
 */

function zsShowNotification(message, success) {
    var notification = document.createElement('div');
    notification.classList.add('notification');
    notification.classList.add(success ? 'success' : 'error');
    notification.textContent = message;
    document.body.appendChild(notification);
    setTimeout(function() {
        if (notification.parentNode) {
            document.body.removeChild(notification);
        }
    }, 1732);
}

document.getElementById('zsCopyReference').addEventListener('click', function(event) {
    event.preventDefault();
    var reference = this.getAttribute('data-ref');
    navigator.clipboard.writeText(reference)
        .then(function() {
            zsShowNotification("Reference copied to clipboard.", true);
        })
        .catch(function(error) {
            console.error("Error copying text: ", error);
            zsShowNotification("Failed to copy reference. Please try again.", false);
        });
  });