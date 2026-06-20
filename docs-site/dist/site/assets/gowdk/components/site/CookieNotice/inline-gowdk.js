const noticeCookie = "gowdk_notice_ack";
  const notice = document.querySelector("[data-cookie-notice]");
  if (notice) {
    const acked = document["cookie"].split("; ").indexOf(noticeCookie + "=1") !== -1;
    if (acked) {
      notice.remove();
    } else {
      notice.hidden = false;
      const dismiss = notice.querySelector("[data-cookie-dismiss]");
      if (dismiss) {
        dismiss.addEventListener("click", () => {
          document["cookie"] = noticeCookie + "=1; path=/; max-age=31536000; samesite=lax";
          notice.remove();
        });
      }
    }
  }
