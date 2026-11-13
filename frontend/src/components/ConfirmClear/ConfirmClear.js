import { Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

/** Asks before clearing something: `what` is { title, text, action } or null. */
export default function ConfirmClear({ what, onClose }) {
  return (
    <Dialog open={Boolean(what)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{what?.title}</DialogTitle>
          <DialogDescription>{what?.text}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">Keep them</Button>
          </DialogClose>
          <Button
            variant="destructive"
            onClick={() => {
              what.action();
              onClose();
            }}
          >
            <Trash2 /> {what?.confirm}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
