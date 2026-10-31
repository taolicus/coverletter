'use strict';

const API = '/api';
const POLL_MS = 4000;
const MIN_PLAYERS = 2;
const ROOM_PATH = /^\/room\/([^/]+)\/?$/;
const ID_PATTERN = /^[23456789abcdefghjkmnpqrstuvwxyz]+$/i;

const STATE_LABELS = {
  waiting: 'Waiting',
  ready: 'Ready',
  in_progress: 'In progress',
  completed: 'Finished',
  abandoned: 'Abandoned',
};

const ERROR_MESSAGES = {
  room_not_found: "We couldn't find that room. Check the link and try again.",
  room_full: 'This room is full. Ask the host to open another one.',
  room_started: 'This game has already started. Ask the host to open a new room.',
  rate_limited: 'Too many requests. Wait a minute and try again.',
  invalid_name: 'Use a display name between 1 and 32 characters.',
  network: "We couldn't reach the server. Check your connection and try again.",
};

const state = {
  player: null,
  room: null,
  roomID: null,
  roomError: null,
  roomNotice: null,
  roomNoticeKind: null,
  roomStale: false,
  roomFatal: false,
  nameEditing: false,
  pollTimer: null,
  roomLoading: false,
  roomRequest: 0,
  lastRoomJSON: '',
};

let toastTimer = null;

function h(tag, props = {}, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(props || {})) {
    if (value === null || value === undefined || value === false) {
      continue;
    }
    if (key === 'class') {
      node.className = value;
    } else if (key === 'text') {
      node.textContent = String(value);
    } else if (key === 'dataset') {
      Object.assign(node.dataset, value);
    } else if (key.startsWith('on') && typeof value === 'function') {
      node.addEventListener(key.slice(2).toLowerCase(), value);
    } else if (key in node) {
      node[key] = value;
    } else {
      node.setAttribute(key, value === true ? '' : String(value));
    }
  }
  appendChildren(node, children);
  return node;
}

function appendChildren(node, children) {
  for (const child of children.flat(Infinity)) {
    if (child === null || child === undefined || child === false) {
      continue;
    }
    node.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
}

function renderApp(...nodes) {
  const app = document.getElementById('app');
  app.replaceChildren(...nodes);
}

async function request(path, options = {}) {
  const { headers = {}, ...rest } = options;
  const init = {
    credentials: 'same-origin',
    ...rest,
    headers: {
      Accept: 'application/json',
      ...(rest.body ? { 'Content-Type': 'application/json' } : {}),
      ...headers,
    },
  };

  let response;
  try {
    response = await fetch(path, init);
  } catch {
    const error = new Error('Could not reach the server.');
    error.code = 'network';
    throw error;
  }

  const isJSON = (response.headers.get('content-type') || '').includes('application/json');
  const data = isJSON ? await response.json().catch(() => null) : null;

  if (!response.ok) {
    const error = new Error(
      data && data.error && data.error.message
        ? data.error.message
        : `Request failed (${response.status}).`,
    );
    error.code = data && data.error && data.error.code ? data.error.code : 'request_failed';
    error.status = response.status;
    throw error;
  }

  return data;
}

function messageFor(error, fallback) {
  if (error && ERROR_MESSAGES[error.code]) {
    return ERROR_MESSAGES[error.code];
  }
  return (error && error.message) || fallback;
}

function showToast(message) {
  const toast = document.getElementById('toast');
  if (!toast) {
    return;
  }
  toast.textContent = message;
  toast.hidden = false;
  toast.classList.add('is-visible');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toast.classList.remove('is-visible');
    setTimeout(() => {
      toast.hidden = true;
    }, 180);
  }, 2600);
}

function navigate(path) {
  if (path === window.location.pathname) {
    renderRoute();
    return;
  }
  window.history.pushState({}, '', path);
  window.scrollTo(0, 0);
  renderRoute();
}

function parseRoomID(raw) {
  const value = String(raw || '').trim();
  if (!value) {
    return null;
  }
  if (ID_PATTERN.test(value)) {
    return value.toLowerCase();
  }
  try {
    const url = new URL(value, window.location.origin);
    const match = url.pathname.match(ROOM_PATH);
    if (match) {
      return decodeURIComponent(match[1]).toLowerCase();
    }
  } catch {
    return null;
  }
  return null;
}

function stateLabel(value) {
  return STATE_LABELS[value] || value;
}

function identityPanel() {
  const panel = h(
    'section',
    { class: 'panel', id: 'identity' },
    h('p', { class: 'eyebrow' }, 'Your name'),
  );

  if (!state.player.displayName || state.nameEditing) {
    const input = h('input', {
      id: 'display-name',
      name: 'displayName',
      type: 'text',
      maxlength: 32,
      autocomplete: 'nickname',
      placeholder: 'e.g. Ada Lovelace',
      value: state.player.displayName || '',
      'aria-describedby': 'name-help',
    });
    const error = h('p', { class: 'form-error', hidden: true, role: 'alert' });
    const form = h(
      'form',
      {
        class: 'name-form',
        onsubmit: async (event) => {
          event.preventDefault();
          await saveName(input, error, form);
        },
      },
      h(
        'label',
        { class: 'field-label', for: 'display-name' },
        state.player.displayName ? 'Change your display name' : 'Choose a display name',
      ),
      h(
        'div',
        { class: 'field-row' },
        input,
        h('button', { class: 'button button-primary', type: 'submit' }, 'Save name'),
      ),
      h('p', { class: 'hint', id: 'name-help' }, '1–32 characters. Other players will see this.'),
      error,
    );
    panel.append(form);
    if (state.player.displayName) {
      panel.append(
        h(
          'button',
          {
            class: 'text-button',
            type: 'button',
            onclick: () => {
              state.nameEditing = false;
              renderRoute();
            },
          },
          'Cancel',
        ),
      );
    }
  } else {
    panel.append(
      h('p', { class: 'identity-name' }, state.player.displayName),
      h('p', { class: 'hint' }, 'This is what other players see.'),
      h(
        'button',
        {
          class: 'button button-secondary',
          type: 'button',
          onclick: () => {
            state.nameEditing = true;
            renderRoute();
          },
        },
        'Change name',
      ),
    );
  }

  return panel;
}

async function saveName(input, error, form) {
  const value = input.value.trim();
  if (value.length < 1 || value.length > 32) {
    error.textContent = 'Use a name between 1 and 32 characters.';
    error.hidden = false;
    input.focus();
    return;
  }

  const button = form.querySelector('button[type="submit"]');
  button.disabled = true;
  button.textContent = 'Saving…';
  error.hidden = true;

  try {
    state.player = await request(`${API}/player`, {
      method: 'PATCH',
      body: JSON.stringify({ displayName: value }),
    });
    state.nameEditing = false;
    showToast('Name saved.');
    if (state.roomID) {
      await refreshRoom({ quiet: true, force: true });
      if (state.room) {
        renderRoomPage();
      }
    } else {
      renderRoute();
    }
  } catch (err) {
    error.textContent = messageFor(err, "Couldn't save that name.");
    error.hidden = false;
    input.focus();
  } finally {
    button.disabled = false;
    button.textContent = 'Save name';
  }
}

function createPanel() {
  const error = h('p', { class: 'form-error', hidden: true, role: 'alert' });
  const button = h(
    'button',
    {
      class: 'button button-primary button-wide',
      type: 'button',
      onclick: async () => {
        button.disabled = true;
        button.textContent = 'Creating…';
        error.hidden = true;
        try {
          const room = await request(`${API}/rooms`, { method: 'POST' });
          navigate(`/room/${encodeURIComponent(room.id)}`);
        } catch (err) {
          error.textContent = messageFor(err, "Couldn't create a room.");
          error.hidden = false;
        } finally {
          button.disabled = false;
          button.textContent = 'Create a room';
        }
      },
    },
    'Create a room',
  );

  return h(
    'section',
    { class: 'panel' },
    h('p', { class: 'eyebrow' }, 'Start a game'),
    h('h2', {}, 'Open a room'),
    h('p', { class: 'hint' }, 'You get seat 1 and a link to share.'),
    button,
    error,
  );
}

function joinPanel() {
  const input = h('input', {
    id: 'room-link',
    name: 'roomLink',
    type: 'text',
    placeholder: 'Paste a room link or code',
    autocomplete: 'off',
    spellcheck: false,
  });
  const error = h('p', { class: 'form-error', hidden: true, role: 'alert' });
  const form = h(
    'form',
    {
      class: 'join-form',
      onsubmit: (event) => {
        event.preventDefault();
        const id = parseRoomID(input.value);
        if (!id) {
          error.textContent = "That doesn't look like a room link or code.";
          error.hidden = false;
          input.focus();
          return;
        }
        error.hidden = true;
        navigate(`/room/${encodeURIComponent(id)}`);
      },
    },
    h('label', { class: 'field-label', for: 'room-link' }, 'Open a room link'),
    h(
      'div',
      { class: 'field-row' },
      input,
      h('button', { class: 'button button-secondary', type: 'submit' }, 'Open room'),
    ),
    error,
  );

  return h(
    'section',
    { class: 'panel' },
    h('h2', {}, 'Join a game'),
    h('p', { class: 'hint' }, 'Use the link the host shared with you.'),
    form,
  );
}

function renderLanding() {
  const intro = h(
    'section',
    { class: 'intro' },
    h('p', { class: 'eyebrow' }, 'A multiplayer game of getting noticed'),
    h('h1', { class: 'display' }, 'Your cover letter ', h('em', {}, 'needs a little drama.')),
    h(
      'p',
      { class: 'lede' },
      'Gather two to four players, share one link, and take turns trying to stand out. No account. No download.',
    ),
    h(
      'ul',
      { class: 'points' },
      h('li', {}, 'Create a room and send the link.'),
      h('li', {}, 'Take a seat on your phone or laptop.'),
      h('li', {}, 'Come back later; the game waits for you.'),
    ),
  );

  const side = h('div', { class: 'stack' }, identityPanel(), createPanel(), joinPanel());
  renderApp(h('div', { class: 'landing-grid' }, intro, side));
}

function renderRoomPage() {
  if (state.roomFatal) {
    renderRoomFatal();
    return;
  }

  if (!state.room) {
    renderApp(
      h(
        'section',
        { class: 'panel' },
        h('p', { class: 'eyebrow' }, 'Room'),
        h('h1', { class: 'room-title' }, 'Opening room…'),
        h('p', { class: 'hint' }, 'Fetching the seats.'),
      ),
    );
    return;
  }

  const room = state.room;
  const occupied = room.seats.filter((seat) => seat.occupied).length;
  const header = h(
    'div',
    { class: 'room-header' },
    h(
      'div',
      {},
      h('p', { class: 'eyebrow' }, 'Lobby'),
      h('h1', { class: 'room-title' }, 'Room'),
      h('p', { class: 'room-code' }, room.id),
    ),
    h(
      'span',
      { class: `badge badge-${room.state}`, id: 'room-state' },
      stateLabel(room.state),
    ),
  );

  const top = h(
    'section',
    { class: 'panel' },
    header,
    h('p', { class: 'room-status' }, statusLine(room)),
    shareBlock(room),
  );

  if (state.roomStale) {
    top.prepend(
      h(
        'p',
        { class: 'notice notice-warning', role: 'status' },
        "Can't reach the server. Retrying…",
      ),
    );
  }
  if (state.roomNotice) {
    top.prepend(
      h(
        'p',
        {
          class: `notice notice-${state.roomNoticeKind || 'success'}`,
          id: 'room-notice',
          role: state.roomNoticeKind === 'error' ? 'alert' : 'status',
        },
        state.roomNotice,
      ),
    );
  }

  const main = h('div', { class: 'stack' }, top, seatsPanel(room, occupied));
  const side = h('aside', { class: 'stack' }, actionPanel(room), identityPanel());
  renderApp(h('div', { class: 'room-layout' }, main, side));
}

function statusLine(room) {
  const occupied = room.seats.filter((seat) => seat.occupied).length;
  switch (room.state) {
    case 'waiting': {
      const needed = Math.max(0, MIN_PLAYERS - occupied);
      if (needed > 0) {
        return `Waiting for ${needed} more ${needed === 1 ? 'player' : 'players'}.`;
      }
      return 'Waiting for the host to start.';
    }
    case 'ready':
      return 'The room is ready. Waiting for the host to start the game.';
    case 'in_progress':
      return 'The game has started.';
    case 'completed':
      return 'The game has finished.';
    default:
      return `Room status: ${room.state}.`;
  }
}

function shareBlock(room) {
  const input = h('input', {
    id: 'share-url',
    class: 'share-url',
    type: 'text',
    readonly: true,
    value: room.shareURL,
    'aria-label': 'Share this room',
  });

  return h(
    'div',
    { class: 'share' },
    h('label', { class: 'field-label', for: 'share-url' }, 'Share this room'),
    h(
      'div',
      { class: 'field-row share-row' },
      input,
      h(
        'button',
        { class: 'button button-secondary', type: 'button', onclick: () => copyShareURL(input) },
        'Copy link',
      ),
    ),
    h('p', { class: 'hint' }, 'Anyone with the link can take an open seat.'),
  );
}

function seatsPanel(room, occupied) {
  const list = h('ul', { class: 'seat-grid', id: 'seats' });

  for (const seat of room.seats) {
    const isYou = room.viewerSeat === seat.number;
    const isHost = room.creatorSeat === seat.number;
    list.append(
      h(
        'li',
        {
          class: [
            'seat',
            seat.occupied ? 'seat-occupied' : 'seat-empty',
            isYou ? 'seat-you' : '',
          ]
            .filter(Boolean)
            .join(' '),
        },
        h(
          'div',
          { class: 'seat-top' },
          h('span', { class: 'seat-number' }, `Seat ${seat.number}`),
          isHost ? h('span', { class: 'tag' }, 'Host') : null,
          isYou ? h('span', { class: 'tag tag-you' }, 'You') : null,
        ),
        h(
          'p',
          { class: seat.occupied ? 'seat-name' : 'seat-name seat-open' },
          seat.occupied ? seat.displayName : 'Open seat',
        ),
      ),
    );
  }

  return h(
    'section',
    { class: 'panel seats-panel', 'aria-labelledby': 'seats-heading' },
    h(
      'div',
      { class: 'panel-heading' },
      h('h2', { id: 'seats-heading' }, 'Seats'),
      h('p', { class: 'hint' }, `${occupied} of 4 taken`),
    ),
    list,
  );
}

function actionPanel(room) {
  const panel = h('section', { class: 'panel action-panel', id: 'room-actions' });

  if (room.viewerSeat !== null && room.viewerSeat !== undefined) {
    const host = room.viewerSeat === room.creatorSeat;
    panel.append(
      h('p', { class: 'eyebrow' }, host ? "You're the host" : "You're seated"),
      h('h2', {}, `Seat ${room.viewerSeat}`),
      h('p', { class: 'hint' }, seatedMessage(room)),
    );
    return panel;
  }

  if (room.state === 'in_progress' || room.state === 'completed') {
    panel.append(
      h('h2', {}, room.state === 'completed' ? 'Game finished' : 'Game in progress'),
      h(
        'p',
        { class: 'hint' },
        room.state === 'completed'
          ? 'This room has finished its game.'
          : "This game has already started. You can watch this page, but you can't take a seat now.",
      ),
    );
    return panel;
  }

  const open = room.seats.find((seat) => !seat.occupied);
  if (!open) {
    panel.append(
      h('h2', {}, 'Room full'),
      h('p', { class: 'hint' }, 'Every seat is taken. Ask the host to open another room.'),
    );
    return panel;
  }

  const button = h(
    'button',
    { class: 'button button-primary button-wide', type: 'button' },
    `Take seat ${open.number}`,
  );
  button.addEventListener('click', () => joinRoom(button, open.number));
  panel.append(
    h('p', { class: 'eyebrow' }, "There's room"),
    h('h2', {}, `Seat ${open.number} is open`),
    h('p', { class: 'hint' }, 'Take it and your browser keeps it for this room.'),
    button,
  );
  return panel;
}

function seatedMessage(room) {
  switch (room.state) {
    case 'waiting':
      return 'Waiting for more players. Share the link.';
    case 'ready':
      return 'The room is ready. Waiting for the host to start.';
    case 'in_progress':
      return 'The game is in progress.';
    case 'completed':
      return 'The game has finished.';
    default:
      return '';
  }
}

async function joinRoom(button, seatNumber) {
  const id = state.roomID;
  if (!id) {
    return;
  }
  button.disabled = true;
  button.textContent = 'Taking seat…';
  state.roomNotice = null;
  state.roomNoticeKind = null;

  try {
    const room = await request(`${API}/rooms/${encodeURIComponent(id)}/join`, {
      method: 'POST',
    });
    if (state.roomID !== id) {
      return;
    }
    state.roomRequest += 1;
    state.roomLoading = false;
    state.room = room;
    state.lastRoomJSON = JSON.stringify(room);
    state.roomStale = false;
    state.roomNotice =
      room.viewerSeat === null || room.viewerSeat === undefined
        ? 'Seat taken.'
        : `You're in seat ${room.viewerSeat}.`;
    state.roomNoticeKind = 'success';
    renderRoomPage();
  } catch (err) {
    if (state.roomID !== id) {
      return;
    }
    state.roomNotice = messageFor(err, "Couldn't take that seat.");
    state.roomNoticeKind = 'error';
    if (err.code === 'room_not_found') {
      state.roomFatal = true;
      state.roomError = state.roomNotice;
      stopPolling();
      renderRoomPage();
      return;
    }
    renderRoomPage();
    if (err.code === 'room_full' || err.code === 'room_started') {
      void refreshRoom({ quiet: true, force: true });
    }
  } finally {
    if (button.isConnected) {
      button.disabled = false;
      button.textContent = `Take seat ${seatNumber}`;
    }
  }
}

async function copyShareURL(input) {
  const value = input.value;
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(value);
    } else {
      input.focus();
      input.select();
      const copied = document.execCommand('copy');
      input.setSelectionRange(0, 0);
      if (!copied) {
        throw new Error('copy failed');
      }
    }
    showToast('Room link copied.');
  } catch {
    input.focus();
    input.select();
    showToast('Select the link and copy it.');
  }
}

async function refreshRoom({ quiet = false, force = false } = {}) {
  const id = state.roomID;
  if (!id || state.roomLoading) {
    return;
  }

  const requestID = state.roomRequest + 1;
  state.roomRequest = requestID;
  state.roomLoading = true;
  const wasStale = state.roomStale;

  try {
    const room = await request(`${API}/rooms/${encodeURIComponent(id)}`);
    if (state.roomRequest !== requestID || state.roomID !== id) {
      return;
    }
    const serialized = JSON.stringify(room);
    const changed = serialized !== state.lastRoomJSON;
    state.room = room;
    state.lastRoomJSON = serialized;
    state.roomError = null;
    state.roomStale = false;
    if (!quiet || changed || force || wasStale) {
      renderRoomPage();
    }
  } catch (err) {
    if (state.roomRequest !== requestID || state.roomID !== id) {
      return;
    }
    if (err.code === 'room_not_found') {
      state.roomFatal = true;
      state.roomError = messageFor(err, "We couldn't find that room.");
      stopPolling();
      renderRoomPage();
      return;
    }
    if (!quiet || !state.room) {
      state.roomError = messageFor(err, "Couldn't load the room.");
      state.roomFatal = true;
      renderRoomPage();
      return;
    }
    state.roomStale = true;
    renderRoomPage();
  } finally {
    if (state.roomRequest === requestID) {
      state.roomLoading = false;
    }
  }
}

function startPolling() {
  stopPolling();
  if (!state.roomID || state.roomFatal) {
    return;
  }
  state.pollTimer = window.setInterval(() => {
    if (document.visibilityState === 'visible') {
      void refreshRoom({ quiet: true });
    }
  }, POLL_MS);
}

function stopPolling() {
  if (state.pollTimer !== null) {
    window.clearInterval(state.pollTimer);
    state.pollTimer = null;
  }
}

function renderRoomFatal() {
  const message = state.roomError || "We couldn't open that room.";
  renderApp(
    h(
      'section',
      { class: 'panel error-panel' },
      h('p', { class: 'eyebrow' }, 'Room unavailable'),
      h('h1', { class: 'room-title' }, "We couldn't open that room."),
      h('p', { class: 'lede' }, message),
      h(
        'div',
        { class: 'button-row' },
        h('a', { class: 'button button-primary', href: '/' }, 'Back to the lobby'),
        h(
          'button',
          {
            class: 'button button-secondary',
            type: 'button',
            onclick: () => {
              state.roomFatal = false;
              state.room = null;
              state.roomError = null;
              state.roomNotice = null;
              state.roomStale = false;
              state.lastRoomJSON = '';
              renderRoute();
            },
          },
          'Try again',
        ),
      ),
    ),
  );
}

function renderFatal(title, message) {
  renderApp(
    h(
      'section',
      { class: 'panel error-panel' },
      h('p', { class: 'eyebrow' }, 'Something went wrong'),
      h('h1', { class: 'room-title' }, title),
      h('p', { class: 'lede' }, message),
      h(
        'button',
        {
          class: 'button button-primary',
          type: 'button',
          onclick: () => window.location.reload(),
        },
        'Reload page',
      ),
    ),
  );
}

function renderNotFound() {
  renderApp(
    h(
      'section',
      { class: 'panel error-panel' },
      h('p', { class: 'eyebrow' }, 'Page not found'),
      h('h1', { class: 'room-title' }, "We couldn't find that page."),
      h('p', { class: 'lede' }, 'Check the address, or head back to the lobby.'),
      h('a', { class: 'button button-primary', href: '/' }, 'Back to the lobby'),
    ),
  );
}

function renderRoute() {
  stopPolling();

  const match = window.location.pathname.match(ROOM_PATH);
  if (match) {
    const id = decodeURIComponent(match[1]);
    if (state.roomID !== id) {
      state.roomRequest += 1;
      state.roomLoading = false;
      state.roomID = id;
      state.room = null;
      state.roomError = null;
      state.roomNotice = null;
      state.roomNoticeKind = null;
      state.roomStale = false;
      state.roomFatal = false;
      state.lastRoomJSON = '';
    }
    document.title = `Room ${id} · Cover Letter`;
    renderRoomPage();
    if (!state.room && !state.roomFatal && !state.roomLoading) {
      void refreshRoom({ quiet: false, force: true });
    }
    startPolling();
    return;
  }

  if (window.location.pathname === '/' || window.location.pathname === '') {
    state.roomRequest += 1;
    state.roomLoading = false;
    state.roomID = null;
    state.room = null;
    state.roomError = null;
    state.roomNotice = null;
    state.roomNoticeKind = null;
    state.roomStale = false;
    state.roomFatal = false;
    state.lastRoomJSON = '';
    document.title = 'Cover Letter';
    renderLanding();
    return;
  }

  state.roomRequest += 1;
  state.roomLoading = false;
  state.roomID = null;
  state.room = null;
  document.title = 'Not found · Cover Letter';
  renderNotFound();
}

async function init() {
  try {
    state.player = await request(`${API}/player`);
  } catch (err) {
    renderFatal("We couldn't start your session.", messageFor(err, 'Reload the page and try again.'));
    return;
  }

  renderRoute();
  window.addEventListener('popstate', renderRoute);
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible' && state.roomID) {
      void refreshRoom({ quiet: true });
    }
  });
}

document.addEventListener('DOMContentLoaded', init, { once: true });
