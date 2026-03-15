import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { UpstreamEntry } from "@/pages/upstream";
import { PostRequest } from "@/util";
import { PlusIcon } from "@phosphor-icons/react";
import { DialogDescription } from "@radix-ui/react-dialog";
import { useState } from "react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";

type AddUpstreamProps = {
  onAdd: (entry: UpstreamEntry) => void;
};

export function AddUpstream({ onAdd }: AddUpstreamProps) {
  const { t } = useTranslation();
  const [newUpstreamIP, setNewUpstreamIP] = useState("");
  const [open, setOpen] = useState(false);
  const [isValidating, setIsValidating] = useState(false);

  const validateUpstream = (value: string): boolean => {
    const trimmed = value.trim();

    const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}:\d+$/;
    const ipv6Regex = /^\[([0-9a-fA-F:]+)\]:\d+$/;

    if (ipv4Regex.test(trimmed)) {
      const [ip] = trimmed.split(":");
      const octets = ip.split(".");
      return octets.every((octet) => {
        const num = parseInt(octet, 10);
        return num >= 0 && num <= 255;
      });
    }

    if (ipv6Regex.test(trimmed)) {
      return true;
    }

    return false;
  };

  const handleSave = async () => {
    if (!validateUpstream(newUpstreamIP)) {
      toast.error(t("upstream.add.invalidFormat"));
      return;
    }

    setIsValidating(true);
    try {
      const [code, response] = await PostRequest("upstream", {
        upstream: newUpstreamIP.trim()
      });

      if (code === 200) {
        toast.success(t("upstream.toasts.added"));
        setOpen(false);
        onAdd({
          dnsPing: t("upstream.reloadToPing"),
          icmpPing: t("upstream.reloadToPing"),
          name: newUpstreamIP.trim(),
          preferred: false,
          upstream: newUpstreamIP.trim()
        });
        setNewUpstreamIP("");
      } else {
        toast.error(response?.message || t("upstream.toasts.addFailed"));
      }
    } catch {
      toast.error(t("upstream.toasts.addFailed"));
    } finally {
      setIsValidating(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !isValidating) {
      handleSave();
    }
  };

  return (
    <div className="mb-6">
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger asChild>
          <Button variant="default">
            <PlusIcon className="mr-2" size={20} />
            {t("upstream.add.button")}
          </Button>
        </DialogTrigger>
        <DialogContent className="lg:w-1/3">
          <DialogHeader>
            <DialogTitle className="text-xl">
              {t("upstream.add.title")}
            </DialogTitle>
          </DialogHeader>
          <DialogDescription className="text-sm text-muted-foreground leading-relaxed space-y-3 pt-2">
            <p>
              {t("upstream.add.description")}
            </p>
            <div className="space-y-1.5">
              <p className="font-medium text-foreground">{t("upstream.add.examples")}</p>
              <div className="space-y-1 text-xs">
                <div className="flex items-center gap-2">
                  <span className="text-muted-foreground">IPv4:</span>
                  <code className="bg-muted px-1 py-0.5 rounded text-foreground">
                    1.1.1.1:53
                  </code>
                  <span className="text-muted-foreground">({t("upstream.add.cloudflare")})</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-muted-foreground">IPv4:</span>
                  <code className="bg-muted px-1 py-0.5 rounded text-foreground">
                    8.8.8.8:53
                  </code>
                  <span className="text-muted-foreground">(Google)</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-muted-foreground">IPv6:</span>
                  <code className="bg-muted px-1 py-0.5 rounded text-foreground">
                    [2606:4700:4700::1111]:53
                  </code>
                </div>
              </div>
            </div>
          </DialogDescription>
          <div className="space-y-4 pt-4">
            <div className="space-y-2">
              <Label htmlFor="ip" className="text-sm font-medium">
                {t("upstream.add.addressLabel")}
              </Label>
              <Input
                id="ip"
                value={newUpstreamIP}
                placeholder={t("upstream.add.addressPlaceholder")}
                onChange={(e) => setNewUpstreamIP(e.target.value)}
                onKeyDown={handleKeyDown}
                className="font-mono text-sm"
                disabled={isValidating}
              />
              <p className="text-xs text-muted-foreground">
                {t("upstream.add.ipv6Hint")}
              </p>
            </div>
          </div>
          <div className="flex gap-3 pt-2">
            <Button
              onClick={handleSave}
              disabled={isValidating || !newUpstreamIP.trim()}
              className="flex-1"
            >
              {isValidating ? t("upstream.add.adding") : t("upstream.add.button")}
            </Button>
            <Button
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={isValidating}
            >
              {t("common.cancel")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
