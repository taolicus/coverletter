'use strict';

const API = '/api';
const POLL_MS = 4000;
const MIN_PLAYERS = 2;
const HISTORY_LIMIT = 12;
const ROOM_PATH = /^\/room\/([^/]+)\/?$/;
const ID_PATTERN = /^[23456789abcdefghjkmnpqrstuvwxyz]+$/i;
const TARGET_CARDS = new Set([1, 2, 3, 5, 6]);

const STATE_LABELS = {
  waiting: 'Waiting',
  ready: 'Ready',
  in_progress: 'In progress',
  completed: 'Finished',
  abandoned: 'Abandoned',
};

const CARD_NAMES = {
  1: 'Reference Check',
  2: 'Informational Interview',
  3: 'Salary Comparison',
  4: 'Personal Day',
  5: 'Reassignment',
  6: 'Counter-Offer',
  7: 'Non-Compete',
  8: 'Dream Offer',
};

const CARD_NOTES = {
  1: "Name another player's card (not this one). A correct guess knocks them out.",
  2: "Privately look at another player's hand.",
  3: 'Compare hands. The lower card is knocked out.',
  4: 'Stay protected until your next turn.',
  5: 'Make any player discard their hand and draw a replacement.',
  6: 'Trade hands with another player.',
  7: 'No effect. You must play this if you also hold Reassignment or Counter-Offer.',
  8: 'You are knocked out when you play or discard this.',
};

const ERROR_MESSAGES = {
  room_not_found: "We couldn't find that room. Check the link and try again.",
  room_full: 'This room is full. Ask the host to open another one.',
  room_started: 'This game has already started. Ask the host to open a new room.',
  room_not_ready: 'At least two players need a seat before the game can start.',
  not_creator: 'Only the host can start this game.',
  not_seated: 'Only seated players can make a move.',
  game_not_started: 'The game has not been dealt yet.',
  game_not_in_progress: 'This game is not accepting moves.',
  stale_version: 'The board changed before your move was submitted. It has been refreshed; choose again.',
  not_your_turn: 'Another player moved first. The board has been refreshed.',
  match_over: 'This match has finished.',
  invalid_move: "That move wasn't allowed. Choose another card or target.",
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
  game: null,
  gameError: null,
  gameStale: false,
  gameLoading: false,
  gameRequest: 0,
  lastGameJSON: '',
  lastMoveAt: null,
  canMove: false,
  selectedCard: null,
  selectedTarget: '',
  selectedGuess: '',
  movePending: false,
  moveError: null,
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

function moveErrorMessage(error) {
  if (error && error.code === 'invalid_move' && error.message) {
    return `That move wasn't allowed: ${error.message}`;
  }
  return messageFor(error, "Couldn't play that card.");
}

function cardName(value) {
  return CARD_NAMES[value] || 'Unknown card';
}

function formatClock(date) {
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function playerAt(game, seat) {
  return game.players.find((player) => player.seat === seat) || null;
}

function playerName(game, seat) {
  const player = playerAt(game, seat);
  return player ? player.displayName || player.name : `Seat ${seat}`;
}

function isStartedRoom(room) {
  return Boolean(room && (room.state === 'in_progress' || room.state === 'completed'));
}

function isGameMode() {
  return Boolean(state.game) || isStartedRoom(state.room);
}

function clearRoomState(id = null) {
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
  state.gameRequest += 1;
  state.gameLoading = false;
  state.game = null;
  state.gameError = null;
  state.gameStale = false;
  state.lastGameJSON = '';
  state.lastMoveAt = null;
  state.canMove = false;
  state.selectedCard = null;
  state.selectedTarget = '';
  state.selectedGuess = '';
  state.movePending = false;
  state.moveError = null;
}

function clearMoveSelection() {
  state.selectedCard = null;
  state.selectedTarget = '';
  state.selectedGuess = '';
  state.moveError = null;
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
      await refreshCurrent({ quiet: true, force: true });
      renderRoomPage();
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

  if (isStartedRoom(state.room)) {
    if (state.game) {
      renderGamePage();
      return;
    }
    if (state.gameError) {
      renderGameUnavailable();
      return;
    }
    renderGameLoading();
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
    if (room.canStart) {
      const button = h(
        'button',
        {
          class: 'button button-primary button-wide',
          id: 'start-game',
          type: 'button',
          onclick: () => startGame(button),
        },
        'Start game',
      );
      panel.append(
        h('p', { class: 'hint' }, 'Everyone is seated. Deal the first round when you are ready.'),
        button,
      );
    }
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
      return room.viewerSeat === room.creatorSeat
        ? 'Everyone is here. Start the game when you are ready.'
        : 'The room is ready. Waiting for the host to start.';
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

async function startGame(button) {
  const id = state.roomID;
  if (!id) {
    return;
  }
  button.disabled = true;
  button.textContent = 'Starting…';
  state.roomNotice = null;
  state.roomNoticeKind = null;

  try {
    const room = await request(`${API}/rooms/${encodeURIComponent(id)}/start`, {
      method: 'POST',
    });
    if (state.roomID !== id) {
      return;
    }
    state.room = room;
    state.lastRoomJSON = JSON.stringify(room);
    state.roomError = null;
    state.roomStale = false;
    await refreshGame({ quiet: false, force: true });
    renderRoomPage();
  } catch (err) {
    if (state.roomID !== id) {
      return;
    }
    state.roomNotice = messageFor(err, "Couldn't start the game.");
    state.roomNoticeKind = 'error';
    if (err.code === 'room_started') {
      await refreshCurrent({ quiet: true, force: true });
    }
    renderRoomPage();
  } finally {
    if (button.isConnected) {
      button.disabled = false;
      button.textContent = 'Start game';
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
    if (isStartedRoom(room)) {
      state.gameError = null;
      state.gameStale = false;
      if (state.gameLoading) {
        renderRoomPage();
      } else {
        await refreshGame({ quiet, force: true });
      }
      return;
    }
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

function applyGameData(data) {
  const previousVersion = state.game ? state.game.version : null;
  const versionChanged = previousVersion !== data.game.version;
  state.room = data.room;
  state.lastRoomJSON = JSON.stringify(data.room);
  state.game = data.game;
  state.canMove = Boolean(data.canMove);
  state.gameError = null;
  state.gameStale = false;
  state.roomError = null;
  state.roomStale = false;
  state.lastGameJSON = JSON.stringify(data);
  if (versionChanged) {
    state.lastMoveAt = previousVersion === null ? null : new Date();
    clearMoveSelection();
  } else {
    const hand = data.game.hand || [];
    if (state.selectedCard && !hand.includes(Number(state.selectedCard))) {
      clearMoveSelection();
    }
  }
}

async function refreshGame({ quiet = false, force = false } = {}) {
  const id = state.roomID;
  if (!id || state.gameLoading) {
    return;
  }

  const requestID = state.gameRequest + 1;
  state.gameRequest = requestID;
  state.gameLoading = true;
  const wasStale = state.gameStale;

  try {
    const data = await request(`${API}/rooms/${encodeURIComponent(id)}/game`);
    if (state.gameRequest !== requestID || state.roomID !== id) {
      return;
    }
    const changed = JSON.stringify(data) !== state.lastGameJSON;
    applyGameData(data);
    if (!quiet || changed || force || wasStale) {
      renderRoomPage();
    }
  } catch (err) {
    if (state.gameRequest !== requestID || state.roomID !== id) {
      return;
    }
    if (err.code === 'room_not_found') {
      state.roomFatal = true;
      state.roomError = messageFor(err, "We couldn't find that room.");
      stopPolling();
      renderRoomPage();
      return;
    }
    if (!quiet || !state.game) {
      state.gameError = messageFor(err, "Couldn't load the game.");
      renderRoomPage();
      return;
    }
    state.gameStale = true;
    renderRoomPage();
  } finally {
    if (state.gameRequest === requestID) {
      state.gameLoading = false;
    }
  }
}

function refreshCurrent(options = {}) {
  if (isGameMode()) {
    return refreshGame(options);
  }
  return refreshRoom(options);
}

function startPolling() {
  stopPolling();
  if (!state.roomID || state.roomFatal) {
    return;
  }
  state.pollTimer = window.setInterval(() => {
    if (document.visibilityState === 'visible') {
      void refreshCurrent({ quiet: true });
    }
  }, POLL_MS);
}

function stopPolling() {
  if (state.pollTimer !== null) {
    window.clearInterval(state.pollTimer);
    state.pollTimer = null;
  }
}

function forcedNonCompete(hand) {
  return hand.includes(7) && (hand.includes(5) || hand.includes(6));
}

function legalTargets(card, game, seat) {
  if (card === 5) {
    return game.players.filter(
      (player) => !player.eliminated && (!player.protected || player.seat === seat),
    );
  }
  if (TARGET_CARDS.has(card)) {
    return game.players.filter(
      (player) => player.seat !== seat && !player.eliminated && !player.protected,
    );
  }
  return [];
}

function moveNeedsTarget(card, game, seat) {
  return TARGET_CARDS.has(card) && legalTargets(card, game, seat).length > 0;
}

function moveNeedsGuess(card, game, seat) {
  return card === 1 && moveNeedsTarget(card, game, seat);
}

function selectedMove() {
  const game = state.game;
  const card = Number(state.selectedCard);
  if (!game || !state.canMove || !card || !(game.hand || []).includes(card)) {
    return null;
  }

  const seat = game.viewerSeat;
  const targetNeeded = moveNeedsTarget(card, game, seat);
  const target = targetNeeded ? Number(state.selectedTarget) : 0;
  if (targetNeeded && !legalTargets(card, game, seat).some((player) => player.seat === target)) {
    return null;
  }

  const guessNeeded = moveNeedsGuess(card, game, seat);
  const guess = guessNeeded ? Number(state.selectedGuess) : 0;
  if (guessNeeded && (guess < 2 || guess > 8)) {
    return null;
  }

  return { version: game.version, card, target, guess };
}

function updateMoveControls() {
  const button = document.getElementById('play-card');
  if (button) {
    button.disabled = state.movePending || !selectedMove();
  }
}

function selectCard(card) {
  if (state.movePending) {
    return;
  }
  state.selectedCard = String(card);
  state.selectedTarget = '';
  state.selectedGuess = '';
  state.moveError = null;
  renderRoomPage();
}

async function submitSelectedMove() {
  if (state.movePending) {
    return;
  }
  const id = state.roomID;
  const move = selectedMove();
  if (!id || !move) {
    return;
  }

  state.movePending = true;
  state.moveError = null;
  renderRoomPage();

  try {
    const data = await request(`${API}/rooms/${encodeURIComponent(id)}/move`, {
      method: 'POST',
      body: JSON.stringify(move),
    });
    if (state.roomID !== id) {
      return;
    }
    applyGameData(data);
    state.movePending = false;
    clearMoveSelection();
    renderRoomPage();
  } catch (err) {
    if (state.roomID !== id) {
      return;
    }
    const message = moveErrorMessage(err);
    state.movePending = false;
    state.moveError = message;
    if (err.code === 'room_not_found') {
      state.roomFatal = true;
      state.roomError = message;
      stopPolling();
      renderRoomPage();
      return;
    }
    if (
      err.code === 'stale_version' ||
      err.code === 'not_your_turn' ||
      err.code === 'game_not_in_progress' ||
      err.code === 'match_over'
    ) {
      state.selectedCard = null;
      state.selectedTarget = '';
      state.selectedGuess = '';
      await refreshGame({ quiet: true, force: true });
      state.moveError = message;
    }
    renderRoomPage();
  }
}

function renderGameLoading() {
  const completed = state.room.state === 'completed';
  renderApp(
    h(
      'section',
      { class: 'panel' },
      h('p', { class: 'eyebrow' }, completed ? 'Final board' : 'Dealing in'),
      h('h1', { class: 'room-title' }, completed ? 'Opening the final board…' : 'Dealing the cards…'),
      h('p', { class: 'hint' }, 'This will only take a moment.'),
    ),
  );
}

function renderGameUnavailable() {
  renderApp(
    h(
      'section',
      { class: 'panel error-panel' },
      h('p', { class: 'eyebrow' }, 'Game unavailable'),
      h('h1', { class: 'room-title' }, "We couldn't load the game."),
      h('p', { class: 'lede' }, state.gameError || 'Try again in a moment.'),
      h(
        'div',
        { class: 'button-row' },
        h('a', { class: 'button button-secondary', href: '/' }, 'Back to the lobby'),
        h(
          'button',
          {
            class: 'button button-primary',
            type: 'button',
            onclick: () => {
              state.gameError = null;
              renderRoomPage();
              void refreshGame({ quiet: false, force: true });
            },
          },
          'Try again',
        ),
      ),
    ),
  );
}

function renderGamePage() {
  const game = state.game;
  const room = state.room;
  const over = game.phase === 'match_over';
  const notices = [];

  if (state.gameStale || state.roomStale) {
    notices.push(
      h(
        'p',
        { class: 'notice notice-warning', role: 'status' },
        "Can't reach the server. Retrying…",
      ),
    );
  }
  if (state.roomNotice) {
    notices.push(
      h(
        'p',
        {
          class: `notice notice-${state.roomNoticeKind || 'success'}`,
          role: state.roomNoticeKind === 'error' ? 'alert' : 'status',
        },
        state.roomNotice,
      ),
    );
  }

  const top = h(
    'section',
    { class: 'panel game-top' },
    h(
      'div',
      { class: 'room-header' },
      h(
        'div',
        {},
        h('p', { class: 'eyebrow' }, over ? 'Match over' : `Round ${game.round}`),
        h('h1', { class: 'room-title' }, over ? 'Final board' : 'Cover Letter'),
        h('p', { class: 'room-code' }, `Room ${room.id}`),
      ),
      h(
        'span',
        { class: `badge badge-${room.state}` },
        stateLabel(room.state),
      ),
    ),
    h(
      'p',
      { class: 'room-status' },
      `Turn ${game.turn} · ${game.drawCount} ${game.drawCount === 1 ? 'card' : 'cards'} left to draw · ${game.target} callbacks to win`,
    ),
    game.faceUp && game.faceUp.length
      ? h(
          'p',
          { class: 'game-meta' },
          'Set aside face up: ',
          game.faceUp.map(cardName).join(', '),
        )
      : null,
    ...notices,
  );

  const main = h(
    'div',
    { class: 'stack' },
    turnPanel(game),
    seatsPanelGame(game),
    historyPanel(game),
  );
  const side = h(
    'aside',
    { class: 'stack' },
    handPanel(game),
    snapshotsPanel(game),
    roundPanel(game),
  );

  renderApp(
    h(
      'div',
      {
        id: 'game-view',
        class: 'game-page',
        dataset: {
          version: String(game.version),
          phase: game.phase,
          canMove: String(state.canMove),
        },
      },
      top,
      h('div', { class: 'game-layout' }, main, side),
    ),
  );
}

function turnPanel(game) {
  if (game.phase === 'match_over') {
    const winner = playerAt(game, game.matchWinnerSeat);
    const name = winner ? winner.name : `Seat ${game.matchWinnerSeat}`;
    const callbacks = winner ? winner.callbacks : game.target;
    return h(
      'section',
      { class: 'panel turn-panel match-over', id: 'match-result' },
      h('p', { class: 'eyebrow' }, 'Match over'),
      h('h2', {}, `${name} wins!`),
      h('p', { class: 'hint' }, `${callbacks} callbacks. The position is filled.`),
    );
  }

  const active = playerAt(game, game.activeSeat);
  const activeName = active ? active.name : 'the next player';
  const viewer = playerAt(game, game.viewerSeat);
  const yourTurn = state.canMove;

  let hint;
  if (yourTurn) {
    hint = 'Choose one card from your hand and play it.';
  } else if (viewer && viewer.eliminated) {
    hint = `You are out of this round. Waiting for ${activeName} to move.`;
  } else {
    hint = `It's ${activeName}'s turn. This board refreshes automatically.`;
  }

  return h(
    'section',
    {
      class: ['panel', 'turn-panel', yourTurn ? 'your-turn' : 'waiting-turn'].join(' '),
      'aria-live': 'polite',
    },
    h('p', { class: 'eyebrow' }, yourTurn ? 'Your move' : 'Waiting'),
    h('h2', { id: 'turn-status' }, yourTurn ? 'Your turn' : `Waiting for ${activeName}`),
    h('p', { class: 'hint' }, hint),
    state.lastMoveAt
      ? h(
          'p',
          { class: 'game-meta', id: 'last-move' },
          `Last move seen at ${formatClock(state.lastMoveAt)}.`,
        )
      : null,
  );
}

function cardChip(card) {
  return h(
    'span',
    { class: 'card-chip' },
    h('span', { class: 'card-chip-rank' }, String(card)),
    h('span', { class: 'card-chip-name' }, cardName(card)),
  );
}

function seatsPanelGame(game) {
  const list = h('ul', { class: 'game-seat-grid', id: 'game-seats' });

  for (const player of game.players) {
    const active = game.phase !== 'match_over' && player.seat === game.activeSeat;
    const you = player.seat === game.viewerSeat;
    const winner = game.phase === 'match_over' && player.seat === game.matchWinnerSeat;
    const classes = ['game-seat'];
    if (active) {
      classes.push('is-active');
    }
    if (you) {
      classes.push('is-you');
    }
    if (player.eliminated) {
      classes.push('is-out');
    }

    const tags = [];
    if (active) {
      tags.push(h('span', { class: 'tag tag-active' }, 'Active'));
    }
    if (you) {
      tags.push(h('span', { class: 'tag tag-you' }, 'You'));
    }
    if (player.eliminated) {
      tags.push(h('span', { class: 'tag tag-out' }, 'Out'));
    } else if (player.protected) {
      tags.push(h('span', { class: 'tag tag-protected' }, 'Protected'));
    }
    if (winner) {
      tags.push(h('span', { class: 'tag tag-winner' }, 'Winner'));
    }

    const handLabel = `${player.handCount} ${player.handCount === 1 ? 'card' : 'cards'} in hand`;
    list.append(
      h(
        'li',
        {
          class: classes.join(' '),
          dataset: {
            seat: String(player.seat),
            active: String(active),
            eliminated: String(player.eliminated),
          },
        },
        h(
          'div',
          { class: 'game-seat-head' },
          h('span', { class: 'seat-number' }, `Seat ${player.seat}`),
          ...tags,
        ),
        h('p', { class: 'game-seat-name' }, player.name),
        h(
          'p',
          { class: 'game-seat-meta' },
          `${player.callbacks} / ${game.target} callbacks · ${handLabel}`,
        ),
        h(
          'div',
          { class: 'discard-block' },
          h('p', { class: 'field-label' }, 'Discards'),
          player.discard && player.discard.length
            ? h('div', { class: 'card-chips' }, player.discard.map(cardChip))
            : h('p', { class: 'hint' }, 'No discards yet.'),
        ),
      ),
    );
  }

  return h(
    'section',
    { class: 'panel', 'aria-labelledby': 'game-seats-heading' },
    h(
      'div',
      { class: 'panel-heading' },
      h('h2', { id: 'game-seats-heading' }, 'The table'),
      h('p', { class: 'hint' }, `${game.playerCount} players`),
    ),
    list,
  );
}

function historyPanel(game) {
  const events = game.history || [];
  const shown = events.slice(-HISTORY_LIMIT).reverse();
  const list = h('ol', { class: 'history-list', id: 'game-history' });

  for (const event of shown) {
    list.append(
      h(
        'li',
        { class: 'history-item', dataset: { kind: event.kind } },
        h(
          'p',
          { class: 'history-meta' },
          `${event.kind === 'play' ? `Round ${event.round} · turn ${event.turn}` : `Round ${event.round}`}`,
        ),
        h('p', { class: 'history-text' }, describeEvent(game, event)),
      ),
    );
  }

  return h(
    'section',
    { class: 'panel', 'aria-labelledby': 'history-heading' },
    h(
      'div',
      { class: 'panel-heading' },
      h('h2', { id: 'history-heading' }, 'Recent history'),
      h('p', { class: 'hint' }, events.length > shown.length ? `Last ${shown.length} of ${events.length}` : 'Newest first'),
    ),
    shown.length ? list : h('p', { class: 'hint' }, 'No moves yet.'),
  );
}

function describeEvent(game, event) {
  const name = event.seat ? playerName(game, event.seat) : '';
  const target = event.target ? playerName(game, event.target) : '';

  if (event.kind === 'round_end') {
    const winners = (event.winners || []).map((seat) => playerName(game, seat)).join(' and ');
    const secret = event.secret ? ` The secret card was ${cardName(event.secret)}.` : '';
    const shared = (event.winners || []).length > 1 ? ' Shared round win.' : '';
    return `${winners || 'No one'} won round ${event.round} and gained a callback.${shared}${secret}`;
  }
  if (event.kind === 'match_end') {
    return `${playerName(game, event.matchWinner)} won the match.`;
  }

  const card = event.card;
  switch (card) {
    case 1:
      if (event.result === 'no_effect') {
        return `${name} played Reference Check. Everyone else was protected, so it had no effect.`;
      }
      if (event.result === 'correct') {
        return `${name} played Reference Check on ${target}, guessing ${cardName(event.guess)}. Correct — ${target} is out.`;
      }
      return `${name} played Reference Check on ${target}, guessing ${cardName(event.guess)}. Wrong guess; no effect.`;
    case 2:
      if (event.result === 'no_effect') {
        return `${name} played Informational Interview. Everyone else was protected, so it had no effect.`;
      }
      return `${name} played Informational Interview on ${target} and viewed their hand.`;
    case 3:
      if (event.result === 'no_effect') {
        return `${name} played Salary Comparison. Everyone else was protected, so it had no effect.`;
      }
      if (event.result === 'actor_eliminated') {
        return `${name} played Salary Comparison with ${target}. ${name} had the lower card and is out.`;
      }
      if (event.result === 'target_eliminated') {
        return `${name} played Salary Comparison with ${target}. ${target} had the lower card and is out.`;
      }
      return `${name} played Salary Comparison with ${target}. The cards tied.`;
    case 4:
      return `${name} played Personal Day and is protected until their next turn.`;
    case 5:
      if (event.result === 'eliminated') {
        return `${name} played Reassignment on ${target}. ${target} discarded Dream Offer and is out.`;
      }
      return `${name} played Reassignment on ${target}. ${target} discarded and drew a replacement.`;
    case 6:
      if (event.result === 'no_effect') {
        return `${name} played Counter-Offer. Everyone else was protected, so it had no effect.`;
      }
      return `${name} played Counter-Offer and swapped hands with ${target}.`;
    case 7:
      return `${name} played Non-Compete. No effect.`;
    case 8:
      return `${name} played Dream Offer and is out.`;
    default:
      return `${name} played ${cardName(card)}.`;
  }
}

function handCardView(card, { selected = false, disabled = false, onSelect = null } = {}) {
  const props = {
    class: ['game-card', 'hand-card', selected ? 'is-selected' : ''].filter(Boolean).join(' '),
    dataset: { card: String(card) },
  };
  const body = [
    h('span', { class: 'card-rank' }, String(card)),
    h(
      'span',
      { class: 'card-body' },
      h('span', { class: 'card-name' }, cardName(card)),
      h('span', { class: 'card-note' }, CARD_NOTES[card]),
    ),
  ];

  if (onSelect) {
    return h(
      'button',
      {
        ...props,
        type: 'button',
        'aria-pressed': selected ? 'true' : 'false',
        disabled,
        onclick: onSelect,
      },
      ...body,
    );
  }
  return h('div', { ...props, 'aria-disabled': 'true' }, ...body);
}

function handPanel(game) {
  const viewer = playerAt(game, game.viewerSeat);
  if (!viewer) {
    return h(
      'section',
      { class: 'panel', id: 'your-hand' },
      h('p', { class: 'eyebrow' }, 'Spectator'),
      h('h2', {}, 'Watching the table'),
      h('p', { class: 'hint' }, 'You do not have a seat in this match. The public board still updates.'),
    );
  }

  const hand = game.hand || [];
  const over = game.phase === 'match_over';
  const forced = forcedNonCompete(hand);
  const interactive = state.canMove && !over && !state.movePending;
  const cards = h(
    'div',
    { class: 'hand-cards' },
    hand.map((card) =>
      handCardView(card, {
        selected: state.selectedCard === String(card),
        disabled: !interactive || (forced && card !== 7),
        onSelect: interactive && (!forced || card === 7) ? () => selectCard(card) : null,
      }),
    ),
  );

  let content;
  if (!hand.length) {
    content = h('p', { class: 'hint' }, 'No cards left this round.');
  } else if (state.canMove && !over) {
    content = [
      forced
        ? h(
            'p',
            { class: 'notice notice-warning' },
            'Non-Compete must be played while you also hold Reassignment or Counter-Offer.',
          )
        : null,
      cards,
      state.selectedCard ? moveControls(game, viewer) : h('p', { class: 'hint' }, 'Choose a card to see its target and effect.'),
    ];
  } else {
    content = [
      h('p', { class: 'hint' }, over ? 'The match is over.' : 'You can play when it is your turn.'),
      cards,
    ];
  }

  return h(
    'section',
    { class: 'panel hand-panel', id: 'your-hand' },
    h(
      'div',
      { class: 'panel-heading' },
      h('h2', {}, 'Your hand'),
      h('p', { class: 'hint' }, `Seat ${viewer.seat} · ${viewer.name}`),
    ),
    state.moveError
      ? h('p', { class: 'form-error', id: 'move-error', role: 'alert' }, state.moveError)
      : null,
    ...content,
  );
}

function moveControls(game, viewer) {
  const card = Number(state.selectedCard);
  const targets = legalTargets(card, game, viewer.seat);
  const targetNeeded = moveNeedsTarget(card, game, viewer.seat);
  const guessNeeded = moveNeedsGuess(card, game, viewer.seat);

  let targetSelect = null;
  if (targetNeeded) {
    targetSelect = h(
      'select',
      {
        id: 'move-target',
        'aria-label': 'Choose a target',
        onchange: (event) => {
          state.selectedTarget = event.target.value;
          updateMoveControls();
        },
      },
      h('option', { value: '' }, 'Choose a player'),
      targets.map((player) =>
        h('option', { value: String(player.seat) }, `${player.name} (seat ${player.seat})`),
      ),
    );
    targetSelect.value = state.selectedTarget;
  }

  let guessSelect = null;
  if (guessNeeded) {
    guessSelect = h(
      'select',
      {
        id: 'move-guess',
        'aria-label': 'Choose a card to guess',
        onchange: (event) => {
          state.selectedGuess = event.target.value;
          updateMoveControls();
        },
      },
      h('option', { value: '' }, 'Choose a card'),
      [2, 3, 4, 5, 6, 7, 8].map((value) =>
        h('option', { value: String(value) }, `${value}. ${cardName(value)}`),
      ),
    );
    guessSelect.value = state.selectedGuess;
  }

  const noTargets =
    TARGET_CARDS.has(card) && targets.length === 0
      ? h('p', { class: 'hint' }, 'Everyone else is protected, so this card will have no effect.')
      : null;

  const button = h(
    'button',
    {
      class: 'button button-primary button-wide',
      id: 'play-card',
      type: 'submit',
      disabled: state.movePending || !selectedMove(),
    },
    state.movePending ? 'Playing…' : 'Play card',
  );

  return h(
    'form',
    {
      class: 'move-controls',
      id: 'move-controls',
      onsubmit: (event) => {
        event.preventDefault();
        void submitSelectedMove();
      },
    },
    h('p', { class: 'eyebrow' }, `Play ${cardName(card)}`),
    targetSelect
      ? h('div', { class: 'move-field' }, h('label', { for: 'move-target' }, 'Target'), targetSelect)
      : null,
    guessSelect
      ? h('div', { class: 'move-field' }, h('label', { for: 'move-guess' }, 'Guess their card'), guessSelect)
      : null,
    noTargets,
    button,
  );
}

function snapshotsPanel(game) {
  const viewer = playerAt(game, game.viewerSeat);
  const snapshots = (viewer && viewer.snapshots) || [];
  if (!snapshots.length) {
    return null;
  }

  return h(
    'section',
    { class: 'panel', id: 'snapshots' },
    h('p', { class: 'eyebrow' }, 'Private notes'),
    h('h2', {}, 'Hands you have seen'),
    h(
      'ul',
      { class: 'snapshot-list' },
      snapshots.map((snapshot) =>
        h(
          'li',
          {},
          h('p', { class: 'history-meta' }, `Round ${snapshot.round} · turn ${snapshot.turn}`),
          h(
            'p',
            { class: 'history-text' },
            `${playerName(game, snapshot.targetSeat)}: ${snapshot.cards.map(cardName).join(', ')}`,
          ),
        ),
      ),
    ),
    h('p', { class: 'hint' }, 'These are snapshots from the moment you looked; later draws do not update them.'),
  );
}

function roundPanel(game) {
  const round = game.lastRound;
  if (!round) {
    return null;
  }
  const winners = (round.winners || []).map((seat) => playerName(game, seat)).join(' and ');
  return h(
    'section',
    { class: 'panel', id: 'last-round' },
    h('p', { class: 'eyebrow' }, `Round ${round.round} result`),
    h('h2', {}, winners ? `${winners} won the round` : 'Round complete'),
    round.secret
      ? h('p', { class: 'hint' }, `Secret card: ${cardName(round.secret)}.`)
      : h('p', { class: 'hint' }, 'The secret card had already been drawn.'),
    round.hands && round.hands.length
      ? h(
          'ul',
          { class: 'snapshot-list' },
          round.hands.map((hand) =>
            h(
              'li',
              {},
              h('p', { class: 'history-text' }, `${playerName(game, hand.seat)}: ${hand.cards.map(cardName).join(', ')}`),
            ),
          ),
        )
      : null,
  );
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
              clearRoomState(state.roomID);
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
      clearRoomState(id);
    }
    document.title = `Room ${id} · Cover Letter`;
    renderRoomPage();
    if (!state.roomFatal) {
      if (isGameMode() && !state.game && !state.gameError && !state.gameLoading) {
        void refreshGame({ quiet: false, force: true });
      } else if (!state.room && !state.roomLoading) {
        void refreshRoom({ quiet: false, force: true });
      }
    }
    startPolling();
    return;
  }

  if (window.location.pathname === '/' || window.location.pathname === '') {
    clearRoomState(null);
    document.title = 'Cover Letter';
    renderLanding();
    return;
  }

  clearRoomState(null);
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
      void refreshCurrent({ quiet: true, force: true });
    }
  });
}

document.addEventListener('DOMContentLoaded', init, { once: true });
