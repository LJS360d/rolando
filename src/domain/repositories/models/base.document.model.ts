import { Document } from "mongoose";

export class BaseDocument extends Document {
  id: string;
  updatedAt: NativeDate

}