// Validate manual bans before submission; the backend remains authoritative.
(() => {
  const ip = document.getElementById("ban-ip");
  const duration = document.getElementById("ban-duration");
  if (!ip || !duration) return;

  // Accept strict IPv4 or bracket-free IPv6 addresses, without ports, CIDRs, or zones.
  const validIP = (value) => {
    if (!value.includes(":")) {
      const parts = value.split(".");
      return (
        parts.length === 4 &&
        parts.every(
          (part) => /^(0|[1-9]\d{0,2})$/.test(part) && Number(part) <= 255,
        )
      );
    }
    if (!/^[\da-fA-F:.]+$/.test(value)) return false;
    try {
      return new URL(`http://[${value}]/`).hostname.startsWith("[");
    } catch {
      return false;
    }
  };

  // Match positive Go durations, including compound units, fractions, and int64 bounds.
  const validDuration = (value) => {
    if (value.startsWith("+")) value = value.slice(1);
    const units = {
      ns: 1n,
      us: 1000n,
      µs: 1000n,
      μs: 1000n,
      ms: 1000000n,
      s: 1000000000n,
      m: 60000000000n,
      h: 3600000000000n,
    };
    const token = /(\d+(?:\.\d*)?|\.\d+)(ns|us|µs|μs|ms|s|m|h)/gy;
    let offset = 0;
    let total = 0n;
    while (offset < value.length) {
      token.lastIndex = offset;
      const match = token.exec(value);
      if (!match) return false;
      const [whole, fraction = ""] = match[1].split(".");
      const scale = 10n ** BigInt(fraction.length);
      total += (BigInt((whole || "0") + fraction) * units[match[2]]) / scale;
      if (total > 9223372036854775807n) return false;
      offset = token.lastIndex;
    }
    return total > 0n;
  };

  // Bind native constraint validation to accessible messages and visual feedback.
  const bind = (field, validate, message) => {
    const hint = document.getElementById(
      field.getAttribute("aria-describedby"),
    );
    let touched = false;
    let animation;

    // Refresh validity on edits while delaying visible feedback until interaction.
    const update = () => {
      const value = field.value.trim();
      const valid = value.length <= field.maxLength && validate(value);
      field.setCustomValidity(valid ? "" : message);
      if (touched) {
        field.setAttribute("aria-invalid", String(!valid));
        hint.textContent = valid ? "" : message;
        hint.hidden = valid;
      }
      return valid;
    };

    // Highlight a rejected field without animation when reduced motion is requested.
    const reject = () => {
      touched = true;
      animation?.cancel();
      if (
        update() ||
        window.matchMedia("(prefers-reduced-motion: reduce)").matches
      )
        return;
      animation = field.animate(
        [
          { transform: "translateX(0)" },
          { transform: "translateX(-5px)" },
          { transform: "translateX(5px)" },
          { transform: "translateX(-3px)" },
          { transform: "translateX(0)" },
        ],
        { duration: 240, easing: "ease-out" },
      );
      animation.id = "validation-shake";
    };

    field.addEventListener("input", update);
    field.addEventListener("blur", reject);
    field.addEventListener("invalid", reject);
    window.addEventListener("pageshow", update);
    update();
  };

  bind(
    ip,
    validIP,
    "Enter a valid IPv4 or IPv6 address, without a port or subnet prefix.",
  );
  bind(
    duration,
    validDuration,
    "Enter a positive duration, such as 30m, 1h, or 1h30m. Days (d) are not supported.",
  );
})();
