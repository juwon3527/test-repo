const form = document.querySelector("#login-form");
const errorBox = document.querySelector("#login-error");
const submitButton = document.querySelector("#login-submit");
const submitLabel = document.querySelector("#submit-label");
const passwordInput = document.querySelector("#password");
const passwordToggle = document.querySelector("#password-toggle");
const signedIn = document.querySelector("#signed-in");
const switchRow = document.querySelector("#login-switch-row");

const modes = {
  signin: {
    eyebrow: "MEMBERS AREA",
    title: "SIGN IN",
    intro: "Pick up where you left off — your fixtures, your journal, your Bridge.",
    submit: "SIGN IN",
    busy: "SIGNING IN…",
    prompt: "New to The Bridge?",
    switchText: "Create an account",
    switchHref: "#signup",
    endpoint: "/api/login",
  },
  signup: {
    eyebrow: "JOIN THE BRIDGE",
    title: "CREATE ACCOUNT",
    intro: "One account for the fan gallery, your profile and everything that comes next.",
    submit: "CREATE ACCOUNT",
    busy: "CREATING ACCOUNT…",
    prompt: "Already a member?",
    switchText: "Sign in",
    switchHref: "#signin",
    endpoint: "/api/signup",
  },
};
let mode = "signin";

function showError(message) {
  errorBox.textContent = message;
  errorBox.hidden = !message;
}

function setMode(next) {
  mode = next;
  const copy = modes[mode];
  const isSignup = mode === "signup";
  document.querySelector("#login-eyebrow").textContent = copy.eyebrow;
  document.querySelector("#login-title").firstChild.textContent = copy.title;
  document.querySelector("#login-intro").textContent = copy.intro;
  document.querySelector("#switch-prompt").textContent = copy.prompt;
  const link = document.querySelector("#mode-switch");
  link.textContent = copy.switchText;
  link.href = copy.switchHref;
  submitLabel.textContent = copy.submit;
  document.querySelector("#name-field").hidden = !isSignup;
  document.querySelector("#password-hint").hidden = !isSignup;
  passwordInput.autocomplete = isSignup ? "new-password" : "current-password";
  document.title = `${isSignup ? "Create account" : "Sign in"} — The Bridge`;
  showError("");
}

function showSignedIn(name) {
  document.querySelector("#signed-in-name").textContent = name;
  signedIn.hidden = false;
  form.hidden = true;
  switchRow.hidden = true;
}

window.addEventListener("hashchange", () => setMode(location.hash === "#signup" ? "signup" : "signin"));
setMode(location.hash === "#signup" ? "signup" : "signin");

passwordToggle.addEventListener("click", () => {
  const reveal = passwordInput.type === "password";
  passwordInput.type = reveal ? "text" : "password";
  passwordToggle.textContent = reveal ? "HIDE" : "SHOW";
  passwordToggle.setAttribute("aria-pressed", String(reveal));
});

function validate(name, email, password) {
  if (mode === "signup" && !name) return "Enter your name.";
  if (!email || !password) return "Enter your email and password.";
  if (!form.email.checkValidity()) return "Enter a valid email address.";
  if (mode === "signup" && password.length < 8) return "Use a password of at least 8 characters.";
  return "";
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const name = document.querySelector("#name").value.trim();
  const email = form.email.value.trim();
  const password = form.password.value;
  const problem = validate(name, email, password);
  if (problem) {
    showError(problem);
    return;
  }

  const copy = modes[mode];
  showError("");
  submitButton.disabled = true;
  submitLabel.textContent = copy.busy;
  try {
    const response = await fetch(copy.endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(mode === "signup" ? { name, email, password } : { email, password }),
    });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || "Something went wrong. Please try again.");
    window.location.assign(mode === "signup" ? "/account.html" : "/");
  } catch (error) {
    showError(error.message);
    form.password.value = "";
    form.password.focus();
  } finally {
    submitButton.disabled = false;
    submitLabel.textContent = copy.submit;
  }
});

fetch("/api/session")
  .then((response) => response.json())
  .then((data) => { if (data.authenticated) showSignedIn(data.name); })
  .catch(() => {});
