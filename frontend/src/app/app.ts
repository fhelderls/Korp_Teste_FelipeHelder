import { Component } from '@angular/core';
import { Produtos } from './produtos/produtos';
import { Notas } from './notas/notas';

@Component({
  selector: 'app-root',
  imports: [Produtos, Notas],
  templateUrl: './app.html',
  styleUrl: './app.css'
})
export class App {
}
