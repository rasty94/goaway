import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from "@/components/ui/table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from "@/components/ui/select";
import { NoContent } from "@/shared";
import { DeleteRequest, GetRequest, PostRequest } from "@/util";
import {
  SpinnerIcon,
  TrashIcon,
  UserCirclePlusIcon,
  UsersIcon
} from "@phosphor-icons/react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

export type UserEntry = {
  username: string;
  role: string;
};

export function Users() {
  const { t } = useTranslation();
  const [users, setUsers] = useState<UserEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState("admin");

  useEffect(() => {
    const loadUsers = async () => {
      setLoading(true);
      const [code, data] = await GetRequest("users");
      if (code !== 200) {
        toast.error(t("users.fetchError"));
        setUsers([]);
      } else {
        setUsers(data || []);
      }
      setLoading(false);
    };

    loadUsers();
  }, [t]);

  const handleCreate = async () => {
    if (!username || !password) {
      toast.warning(t("users.requiredFields"));
      return;
    }

    setSubmitting(true);
    const [code, response] = await PostRequest("users", {
      username,
      password,
      role
    }, true, true);

    if (code === 201) {
      toast.success(t("users.successCreate", { username }));
      setUsers(users.concat({ username, role }));
      setUsername("");
      setPassword("");
      setRole("admin");
    } else {
      toast.error(response.error || t("users.createError"));
    }
    setSubmitting(false);
  };

  const handleDelete = async (userToDelete: string) => {
    if (userToDelete === "admin") {
      toast.error(t("users.cannotDeleteAdmin"));
      return;
    }

    const [code, response] = await DeleteRequest(`users/${userToDelete}`, null);
    if (code === 200) {
      toast.success(t("users.successDelete", { username: userToDelete }));
      setUsers((prev) => prev.filter((u) => u.username !== userToDelete));
    } else {
      toast.error(response.error || t("users.deleteError"));
    }
  };

  return (
    <div className="flex justify-center items-center">
      <div className="space-y-6 w-full xl:w-2/3">
        <div className="flex flex-col gap-2 sm:flex-row sm:justify-between sm:items-center">
          <div>
            <h1 className="text-2xl sm:text-3xl lg:text-4xl font-bold">{t("users.title")}</h1>
            <p className="text-muted-foreground text-sm">
              {t("users.description")}
            </p>
          </div>
          <div className="flex items-center gap-2 text-sm">
            <UsersIcon className="h-4 w-4" />
            {users.length} {users.length === 1 ? t("whitelist.entry") : t("whitelist.entries")}
          </div>
        </div>

        <Card>
          <CardHeader className="pb-4 border-b-2">
            <CardTitle className="flex items-center gap-2">
              <UserCirclePlusIcon className="h-5 w-5 text-green-500" />
              {t("users.newUser")}
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-6">
            <div className="grid gap-6 md:grid-cols-3">
              <div className="space-y-2">
                <Label htmlFor="username">{t("users.username")}</Label>
                <Input
                  id="username"
                  placeholder="admin"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">{t("users.password")}</Label>
                <Input
                  id="password"
                  type="password"
                  placeholder="********"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="role">{t("users.role")}</Label>
                <Select value={role} onValueChange={setRole}>
                  <SelectTrigger id="role">
                    <SelectValue placeholder={t("users.role")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="admin">{t("users.admin")}</SelectItem>
                    <SelectItem value="viewer">{t("users.viewer")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            
            <div className="mt-6 flex flex-col gap-4">
               <div className="text-sm text-muted-foreground p-3 bg-accent/30 rounded-md border border-accent">
                 {role === "admin" ? t("users.adminHint") : t("users.viewerHint")}
               </div>
               <Button
                className="w-full sm:w-auto self-end bg-green-600 hover:bg-green-700"
                onClick={handleCreate}
                disabled={submitting || !username || !password}
              >
                {submitting ? (
                  <>
                    <SpinnerIcon className="h-4 w-4 mr-2 animate-spin" />
                    {t("users.creating")}
                  </>
                ) : (
                  t("users.create")
                )}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-4 border-b-2">
            <CardTitle className="flex items-center gap-2">
              <UsersIcon className="h-5 w-5 text-blue-500" />
              {t("users.title")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="p-6 space-y-4">
                {[1, 2].map((i) => (
                  <Skeleton key={i} className="h-12 w-full" />
                ))}
              </div>
            ) : users.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("users.username")}</TableHead>
                    <TableHead>{t("users.role")}</TableHead>
                    <TableHead className="text-right">Acción</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {users.map((u) => (
                    <TableRow key={u.username}>
                      <TableCell className="font-medium">{u.username}</TableCell>
                      <TableCell>
                        <span className={`px-2 py-1 rounded-full text-xs font-semibold ${
                          u.role === "admin" ? "bg-green-900/30 text-green-400 border border-green-800" : "bg-blue-900/30 text-blue-400 border border-blue-800"
                        }`}>
                          {u.role === "admin" ? t("users.admin") : t("users.viewer")}
                        </span>
                      </TableCell>
                      <TableCell className="text-right">
                        {u.username !== "admin" && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="text-red-500 hover:text-red-700 h-8 w-8 p-0"
                            onClick={() => handleDelete(u.username)}
                          >
                            <TrashIcon className="h-4 w-4" />
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <NoContent text="No hay usuarios configurados" />
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
