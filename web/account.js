const avatarForm = document.querySelector("#avatar-form");
const avatarInput = document.querySelector("#avatar-input");
const avatarError = document.querySelector("#avatar-error");
const avatarSuccess = document.querySelector("#avatar-success");
const avatarSubmit = document.querySelector("#avatar-submit");
const avatarSubmitLabel = document.querySelector("#avatar-submit-label");
const maxImageBytes = 5 * 1024 * 1024;

function initials(name) {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]).join("").toUpperCase();
}

function renderAvatar(element, user) {
  element.replaceChildren();
  if (user.avatarUrl) {
    const image = document.createElement("img");
    image.src = user.avatarUrl;
    image.alt = "";
    element.append(image);
  } else {
    element.textContent = initials(user.name);
  }
}

function renderProfile(user) {
  document.querySelector("#account-name").textContent = user.name;
  document.querySelector("#account-email").textContent = user.email;
  renderAvatar(document.querySelector("#avatar"), user);
}

function showAvatarError(message) {
  avatarError.textContent = message;
  avatarError.hidden = !message;
  if (message) avatarSuccess.hidden = true;
}

avatarInput.addEventListener("change", () => {
  const file = avatarInput.files[0];
  document.querySelector("#avatar-file-name").textContent = file ? file.name : "CHOOSE A PROFILE PHOTO";
  showAvatarError(file && file.size > maxImageBytes ? "Images must be 5 MB or smaller." : "");
  avatarSuccess.hidden = true;
});

avatarForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const file = avatarInput.files[0];
  if (!file) return showAvatarError("Choose an image to upload.");
  if (file.size > maxImageBytes) return showAvatarError("Images must be 5 MB or smaller.");

  showAvatarError("");
  avatarSubmit.disabled = true;
  avatarSubmitLabel.textContent = "UPLOADING…";
  try {
    const body = new FormData();
    body.append("image", file);
    const response = await fetch("/api/account/avatar", { method: "POST", body });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || "Upload failed. Please try again.");
    renderProfile(data);
    avatarForm.reset();
    document.querySelector("#avatar-file-name").textContent = "CHOOSE A PROFILE PHOTO";
    avatarSuccess.hidden = false;
  } catch (error) {
    showAvatarError(error.message);
  } finally {
    avatarSubmit.disabled = false;
    avatarSubmitLabel.textContent = "UPLOAD PHOTO";
  }
});

document.querySelector("#sign-out").addEventListener("click", async () => {
  await fetch("/api/logout", { method: "POST" });
  window.location.assign("/");
});

fetch("/api/session")
  .then((response) => response.json())
  .then((data) => {
    if (!data.authenticated) {
      window.location.replace("/login.html");
      return;
    }
    renderProfile(data);
  })
  .catch(() => showAvatarError("Could not load your account. Please refresh."));
