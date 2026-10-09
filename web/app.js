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
async function loadAccount() {
  const account = document.querySelector("#header-account");
  try {
    const response = await fetch("/api/session");
    const data = await response.json();
    if (!data.authenticated) return;
    const signOut = makeElement("button", "header-account", "SIGN OUT");
    signOut.type = "button";
    signOut.prepend(makeElement("small", "", data.name.toUpperCase()));
    signOut.addEventListener("click", async () => {
      await fetch("/api/logout", { method: "POST" });
      window.location.reload();
    });
    account.replaceWith(signOut);
  } catch (error) {
    console.error(error);
  }
}

loadAccount();
