const fixtureList = document.querySelector("#fixture-list");
const storyGrid = document.querySelector("#story-grid");

function makeElement(tag, className, text) {
  const element = document.createElement(tag);
  if (className) element.className = className;
  if (text) element.textContent = text;
  return element;
}

function renderFixtures(fixtures) {
  fixtureList.replaceChildren(...fixtures.map((fixture, index) => {
    const row = makeElement("article", "fixture-row");
    row.style.animationDelay = `${index * 70}ms`;
      const opponent = makeElement("div", "fixture-opponent");
      row.append(
        makeElement("span", "fixture-comp", fixture.competition),
        opponent,
      );
    opponent.append(makeElement("span", "fixture-crest", fixture.code), document.createTextNode(fixture.opponent));
    row.append(
      makeElement("span", "fixture-date", fixture.date),
      makeElement("span", "fixture-time", fixture.time),
      makeElement("span", "fixture-venue", fixture.venue),
    );
    const link = makeElement("a", "fixture-arrow", "↗");
    link.href = "#matchday";
    link.setAttribute("aria-label", `Match details: Chelsea versus ${fixture.opponent}`);
    row.append(link);
    return row;
  }));
}

function renderStories(articles) {
  storyGrid.replaceChildren(...articles.map((article, index) => {
    const card = makeElement("article", "story-card");
    card.style.animationDelay = `${index * 90}ms`;
    const imageFrame = makeElement("div", "story-image");
    const image = document.createElement("img");
    image.src = article.image;
    image.alt = "Football matchday scene";
    image.loading = "lazy";
    imageFrame.append(image, makeElement("span", "story-category", article.category));
    const copy = makeElement("div", "story-copy");
    copy.append(makeElement("h3", "", article.title), makeElement("span", "story-meta", article.readTime));
    card.append(imageFrame, copy);
    return card;
  }));
}

async function loadMatchCentre() {
  try {
    const response = await fetch("/api/match-centre");
    if (!response.ok) throw new Error(`Request failed: ${response.status}`);
    const data = await response.json();
    document.querySelector("#home-name").textContent = data.match.home.toUpperCase();
    document.querySelector("#away-name").textContent = data.match.away.toUpperCase();
    document.querySelector("#away-code").textContent = data.match.awayCode;
    document.querySelector("#home-score").textContent = data.match.homeScore;
    document.querySelector("#away-score").textContent = data.match.awayScore;
    document.querySelector("#match-status").textContent = data.match.status;
    document.querySelector("#match-venue").textContent = data.match.venue.toUpperCase();
    renderFixtures(data.fixtures);
    renderStories(data.news);
  } catch (error) {
    fixtureList.replaceChildren(makeElement("p", "loading-line", "Match information is unavailable right now."));
    storyGrid.replaceChildren(makeElement("p", "loading-line", "Stories are unavailable right now."));
    console.error(error);
  }
}

const menuToggle = document.querySelector(".menu-toggle");
const mainNav = document.querySelector(".main-nav");
menuToggle.addEventListener("click", () => {
  const isOpen = menuToggle.getAttribute("aria-expanded") === "true";
  menuToggle.setAttribute("aria-expanded", String(!isOpen));
  menuToggle.setAttribute("aria-label", isOpen ? "Open navigation" : "Close navigation");
  mainNav.classList.toggle("open", !isOpen);
});
mainNav.addEventListener("click", (event) => {
  if (event.target.closest("a")) {
    mainNav.classList.remove("open");
    menuToggle.setAttribute("aria-expanded", "false");
    menuToggle.setAttribute("aria-label", "Open navigation");
  }
});

loadMatchCentre();
const maxImageBytes = 5 * 1024 * 1024;
const galleryGrid = document.querySelector("#gallery-grid");
const galleryForm = document.querySelector("#gallery-upload");
const galleryError = document.querySelector("#gallery-error");
let currentUser = null;

function initials(name) {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]).join("").toUpperCase();
}

function makeAvatar(user, className) {
  const avatar = makeElement("span", `avatar ${className}`);
  avatar.setAttribute("aria-hidden", "true");
  if (user.avatarUrl) {
    const image = document.createElement("img");
    image.src = user.avatarUrl;
    image.alt = "";
    avatar.append(image);
  } else {
    avatar.textContent = initials(user.name);
  }
  return avatar;
}

function renderGallery(photos) {
  if (photos.length === 0) {
    galleryGrid.replaceChildren(makeElement("p", "loading-line", "No photos yet. Be the first to share one."));
    return;
  }
  galleryGrid.replaceChildren(...photos.map((photo, index) => {
    const card = makeElement("figure", "gallery-card");
    card.style.animationDelay = `${Math.min(index, 8) * 60}ms`;
    const image = document.createElement("img");
    image.src = photo.imageUrl;
    image.alt = photo.caption || `Matchday photo shared by ${photo.author}`;
    image.loading = "lazy";
    const caption = makeElement("figcaption", "gallery-caption-text");
    if (photo.caption) caption.append(makeElement("span", "", photo.caption));
    caption.append(makeElement("small", "", `BY ${photo.author.toUpperCase()}`));
    card.append(image, caption);
    return card;
  }));
}

async function loadGallery() {
  try {
    const response = await fetch("/api/gallery");
    if (!response.ok) throw new Error(`Request failed: ${response.status}`);
    const data = await response.json();
    renderGallery(data.photos);
    return data.uploadsEnabled;
  } catch (error) {
    galleryGrid.replaceChildren(makeElement("p", "loading-line", "The gallery is unavailable right now."));
    console.error(error);
    return false;
  }
}

function showGalleryError(message) {
  galleryError.textContent = message;
  galleryError.hidden = !message;
}

document.querySelector("#gallery-input").addEventListener("change", (event) => {
  const file = event.target.files[0];
  document.querySelector("#gallery-file-name").textContent = file ? file.name : "CHOOSE A MATCHDAY PHOTO";
  showGalleryError(file && file.size > maxImageBytes ? "Images must be 5 MB or smaller." : "");
});

galleryForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const file = document.querySelector("#gallery-input").files[0];
  if (!file) return showGalleryError("Choose an image to upload.");
  if (file.size > maxImageBytes) return showGalleryError("Images must be 5 MB or smaller.");

  const submit = document.querySelector("#gallery-submit");
  const label = document.querySelector("#gallery-submit-label");
  showGalleryError("");
  submit.disabled = true;
  label.textContent = "POSTING…";
  try {
    const body = new FormData();
    body.append("caption", document.querySelector("#gallery-caption").value);
    body.append("image", file);
    const response = await fetch("/api/gallery", { method: "POST", body });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || "Upload failed. Please try again.");
    galleryForm.reset();
    document.querySelector("#gallery-file-name").textContent = "CHOOSE A MATCHDAY PHOTO";
    await loadGallery();
  } catch (error) {
    showGalleryError(error.message);
  } finally {
    submit.disabled = false;
    label.textContent = "POST PHOTO";
  }
});

async function loadAccount() {
  try {
    const response = await fetch("/api/session");
    const data = await response.json();
    if (!data.authenticated) return;
    currentUser = data;
    const account = makeElement("a", "header-account header-profile");
    account.href = "/account.html";
    account.setAttribute("aria-label", `Your account: ${data.name}`);
    account.append(makeAvatar(data, "avatar-small"), makeElement("span", "header-profile-name", data.name.split(" ")[0].toUpperCase()));
    document.querySelector("#header-account").replaceWith(account);
  } catch (error) {
    console.error(error);
  }
}

async function loadMembersArea() {
  const [, uploadsEnabled] = await Promise.all([loadAccount(), loadGallery()]);
  const cta = document.querySelector("#gallery-cta");
  if (currentUser && uploadsEnabled) {
    galleryForm.hidden = false;
    cta.hidden = true;
  } else if (currentUser) {
    cta.replaceChildren(document.createTextNode("UPLOADS COMING SOON"));
    cta.removeAttribute("href");
  }
}

loadMembersArea();
